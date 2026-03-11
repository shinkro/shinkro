//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/shinkro/shinkro/internal/api"
	"github.com/shinkro/shinkro/internal/auth"
	"github.com/shinkro/shinkro/internal/config"
	"github.com/shinkro/shinkro/internal/database"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"
	"github.com/shinkro/shinkro/internal/user"
	"github.com/shinkro/shinkro/pkg/sse"
)

// setupTestHTTPServer creates a test HTTP server with all services wired up
func setupTestHTTPServer(t *testing.T) (*httptest.Server, *database.DB, api.Service, func()) {
	tmpDir := t.TempDir()
	log := zerolog.Nop()

	// Setup database
	db := database.NewDB(tmpDir, &log)
	require.NotNil(t, db)

	err := db.Migrate()
	require.NoError(t, err)

	// Setup SSE
	serverEvents := sse.New()
	serverEvents.CreateStreamWithOpts("logs", sse.StreamOpts{
		MaxEntries: 1000,
		AutoReplay: true,
	})

	// Initialize repositories
	userRepo := database.NewUserRepo(log, db)
	apiRepo := database.NewAPIRepo(log, db)

	// Initialize services
	userService := user.NewService(userRepo, log)
	authService := auth.NewService(log, userService)
	apiService := api.NewService(log, apiRepo)

	// Create test config
	cfg := &config.AppConfig{
		Config: &domain.Config{
			BaseUrl:       "/",
			SessionSecret: "test-secret-key-for-integration-tests-only",
		},
	}

	// Create HTTP server
	httpServer := NewServer(
		log,
		cfg,
		db,
		"test",
		"test-commit",
		"test-date",
		nil, // plexService - not needed for auth tests
		nil, // plexsettingsService
		nil, // malauthService
		nil, // anilistAuthService
		apiService,
		authService,
		nil, // mappingService
		nil, // fsService
		nil, // notificationService
		nil, // animeUpdateService
		serverEvents,
	)

	// Create test server
	ts := httptest.NewServer(httpServer.Handler())

	cleanup := func() {
		ts.Close()
		db.Close()
	}

	return ts, db, apiService, cleanup
}

func TestAuthIntegration_LoginFlow(t *testing.T) {
	ts, db, _, cleanup := setupTestHTTPServer(t)
	defer cleanup()

	log := zerolog.Nop()
	userRepo := database.NewUserRepo(log, db)
	userService := user.NewService(userRepo, log)
	authService := auth.NewService(log, userService)
	ctx := context.Background()

	// Create user once before all subtests (only 1 user is supported)
	req := testdata.NewMockCreateUserRequest()
	err := authService.CreateUser(ctx, req)
	require.NoError(t, err)

	t.Run("successful login creates session", func(t *testing.T) {

		// Login request
		loginData := domain.User{
			Username: req.Username,
			Password: req.Password,
		}
		body, err := json.Marshal(loginData)
		require.NoError(t, err)

		// Make HTTP request to test server
		resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify response
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify session cookie was set
		cookies := resp.Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "user_session" {
				sessionCookie = cookie
				break
			}
		}
		require.NotNil(t, sessionCookie, "session cookie should be set")
		assert.True(t, sessionCookie.HttpOnly, "session cookie should be HttpOnly")
	})

	t.Run("login with invalid credentials fails", func(t *testing.T) {
		// Login with wrong password (user already created above)
		loginData := domain.User{
			Username: req.Username,
			Password: "wrong-password",
		}
		body, err := json.Marshal(loginData)
		require.NoError(t, err)

		// Make HTTP request
		resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify response
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("validate endpoint requires authentication", func(t *testing.T) {
		// Login (user already created above)
		loginData := domain.User{
			Username: req.Username,
			Password: req.Password,
		}
		body, err := json.Marshal(loginData)
		require.NoError(t, err)

		// Login
		loginResp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, loginResp.StatusCode)

		// Get session cookie
		cookies := loginResp.Cookies()
		var sessionCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "user_session" {
				sessionCookie = cookie
				break
			}
		}
		require.NotNil(t, sessionCookie)

		// Try to validate with session
		validateReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/auth/validate", nil)
		require.NoError(t, err)
		validateReq.AddCookie(sessionCookie)

		client := &http.Client{}
		validateResp, err := client.Do(validateReq)
		require.NoError(t, err)
		defer validateResp.Body.Close()

		// Should succeed with valid session
		assert.Equal(t, http.StatusNoContent, validateResp.StatusCode)
	})
}

func TestAuthIntegration_APIKeyFlow(t *testing.T) {
	ts, _, apiService, cleanup := setupTestHTTPServer(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("API key authentication works", func(t *testing.T) {
		// Create API key (Store generates the key automatically)
		key := &domain.APIKey{
			Name:   "Test Key",
			Scopes: []string{"read", "write"},
		}
		err := apiService.Store(ctx, key)
		require.NoError(t, err)
		require.NotEmpty(t, key.Key, "API key should be generated")

		// Populate cache by calling List (this loads keys into cache)
		_, err = apiService.List(ctx)
		require.NoError(t, err)

		// Verify API key is valid
		valid := apiService.ValidateAPIKey(ctx, key.Key)
		assert.True(t, valid, "API key should be valid")

		// Verify invalid API key is rejected
		valid = apiService.ValidateAPIKey(ctx, "invalid-key")
		assert.False(t, valid, "Invalid API key should be rejected")
	})

	t.Run("API key in header authenticates request", func(t *testing.T) {
		// Create API key (Store generates the key automatically)
		key := &domain.APIKey{
			Name:   "Header Test Key",
			Scopes: []string{"read"},
		}
		err := apiService.Store(ctx, key)
		require.NoError(t, err)
		require.NotEmpty(t, key.Key, "API key should be generated")

		// Populate cache by calling List (this loads keys into cache)
		_, err = apiService.List(ctx)
		require.NoError(t, err)

		// Make request with API key in header
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/config", nil)
		require.NoError(t, err)
		req.Header.Set("Shinkro-Api-Key", key.Key)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should not be 401/403 (authentication should pass)
		// Note: May get 404 or other errors, but not auth errors
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})
}
