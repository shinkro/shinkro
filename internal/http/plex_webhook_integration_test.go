//go:build integration

package http

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/shinkro/shinkro/internal/anime"
	"github.com/shinkro/shinkro/internal/animeupdate"
	"github.com/shinkro/shinkro/internal/api"
	"github.com/shinkro/shinkro/internal/auth"
	"github.com/shinkro/shinkro/internal/config"
	"github.com/shinkro/shinkro/internal/database"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/events"
	"github.com/shinkro/shinkro/internal/malauth"
	"github.com/shinkro/shinkro/internal/mapping"
	"github.com/shinkro/shinkro/internal/notification"
	"github.com/shinkro/shinkro/internal/plex"
	"github.com/shinkro/shinkro/internal/plexsettings"
	"github.com/shinkro/shinkro/internal/testdata"
	"github.com/shinkro/shinkro/internal/user"
	"github.com/shinkro/shinkro/pkg/sse"
)

// setupTestHTTPServerForPlex creates a test HTTP server with all services for Plex webhook testing
func setupTestHTTPServerForPlex(t *testing.T) (*httptest.Server, *database.DB, *EventBus.Bus, api.Service, func()) {
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

	// Setup event bus
	bus := EventBus.New()

	// Initialize repositories
	animeRepo := database.NewAnimeRepo(log, db)
	animeUpdateRepo := database.NewAnimeUpdateRepo(log, db)
	plexRepo := database.NewPlexRepo(log, db)
	plexSettingsRepo := database.NewPlexSettingsRepo(log, db)
	malauthRepo := database.NewMalAuthRepo(log, db)
	userRepo := database.NewUserRepo(log, db)
	apiRepo := database.NewAPIRepo(log, db)
	mappingRepo := database.NewMappingRepo(log, db)
	notificationRepo := database.NewNotificationRepo(log, db)

	// Create test config with proper encryption key (32 bytes hex-encoded)
	testConfig := &domain.Config{
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 64 hex chars = 32 bytes
	}

	// Initialize services
	animeService := anime.NewService(log, animeRepo)
	malauthService := malauth.NewService(testConfig, log, malauthRepo)
	mapService := mapping.NewService(log, mappingRepo)
	plexSettingsService := plexsettings.NewService(testConfig, log, plexSettingsRepo)
	notificationService := notification.NewService(log, notificationRepo)
	animeUpdateService := animeupdate.NewService(log, animeUpdateRepo, animeService, mapService, malauthService, nil, bus)
	plexService := plex.NewService(log, plexSettingsService, plexRepo, animeService, mapService, malauthService, animeUpdateService, bus)
	userService := user.NewService(userRepo, log)
	authService := auth.NewService(log, userService)
	apiService := api.NewService(log, apiRepo)

	// Register event subscribers
	events.NewSubscribers(log, bus, notificationService, plexService, animeUpdateService)

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
		plexService,
		plexSettingsService,
		malauthService,
		nil, // anilistAuthService
		apiService,
		authService,
		mapService,
		nil, // fsService
		notificationService,
		animeUpdateService,
		serverEvents,
	)

	// Create test server
	ts := httptest.NewServer(httpServer.Handler())

	cleanup := func() {
		ts.Close()
		db.Close()
	}

	return ts, db, &bus, apiService, cleanup
}

func TestPlexWebhookIntegration_Flow(t *testing.T) {
	ts, db, _, apiService, cleanup := setupTestHTTPServerForPlex(t)
	defer cleanup()

	ctx := context.Background()
	log := zerolog.Nop()

	// Setup: Create plex settings
	testConfig := &domain.Config{
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 64 hex chars = 32 bytes
	}
	plexSettingsRepo := database.NewPlexSettingsRepo(log, db)
	plexSettingsService := plexsettings.NewService(testConfig, log, plexSettingsRepo)

	// Generate a 12-byte IV for AES-GCM (required for encryption)
	tokenIV := make([]byte, 12)
	for i := range tokenIV {
		tokenIV[i] = byte(i)
	}

	plexSettings := domain.PlexSettings{
		Host:              "localhost",
		Port:              32400,
		TLS:               false,
		PlexUser:          "TestUser",
		AnimeLibraries:    []string{"Anime"},
		PlexClientEnabled: false,
		Token:             []byte("test-token"),
		TokenIV:           tokenIV,
	}
	_, err := plexSettingsService.Store(ctx, plexSettings)
	require.NoError(t, err)

	// Setup: Store anime in database for mapping lookup
	animeRepo := database.NewAnimeRepo(log, db)
	anime := []*domain.Anime{
		{
			MALId:     1575,
			TVDBId:    81797,
			MainTitle: "One Piece",
		},
	}
	err = animeRepo.StoreMultiple(anime)
	require.NoError(t, err)

	t.Run("plex webhook to anime update flow", func(t *testing.T) {
		// Create Plex webhook payload
		payload := testdata.RawPlexWebhookHAMAEpisode()

		// Create multipart form request
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		err = writer.WriteField("payload", payload)
		require.NoError(t, err)
		writer.Close()

		// Create API key for authentication
		key := &domain.APIKey{
			Name:   "Test API Key",
			Scopes: []string{"read", "write"},
		}
		err = apiService.Store(ctx, key)
		require.NoError(t, err)
		require.NotEmpty(t, key.Key, "API key should be generated")

		// Populate cache
		_, err = apiService.List(ctx)
		require.NoError(t, err)

		// Make HTTP request with API key authentication
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/plex", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Shinkro-Api-Key", key.Key)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify response
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Wait a bit for async processing
		time.Sleep(200 * time.Millisecond)

		// Verify Plex payload was stored in database
		plexRepo := database.NewPlexRepo(log, db)
		plexPayloads, err := plexRepo.GetRecent(ctx, 1)
		require.NoError(t, err)
		require.Greater(t, len(plexPayloads), 0, "Plex payload should be stored")

		plexPayload := plexPayloads[0]
		assert.Equal(t, domain.PlexScrobbleEvent, plexPayload.Event)
		assert.Equal(t, domain.PlexWebhook, plexPayload.Source)
		assert.NotEqual(t, int64(0), plexPayload.ID)

		// Verify Plex processing status
		// Note: Since we don't have MAL auth configured, the processing will fail
		// but the plex payload should still be stored
		// The success/failure status depends on whether mapping is found
	})
}

func TestPlexWebhookIntegration_EventFlow(t *testing.T) {
	ts, db, bus, apiService, cleanup := setupTestHTTPServerForPlex(t)
	defer cleanup()

	ctx := context.Background()
	log := zerolog.Nop()

	// Setup: Create plex settings
	testConfig := &domain.Config{
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 64 hex chars = 32 bytes
	}
	plexSettingsRepo := database.NewPlexSettingsRepo(log, db)
	plexSettingsService := plexsettings.NewService(testConfig, log, plexSettingsRepo)

	// Generate a 12-byte IV for AES-GCM (required for encryption)
	tokenIV := make([]byte, 12)
	for i := range tokenIV {
		tokenIV[i] = byte(i + 10) // Different IV for second test
	}

	plexSettings := domain.PlexSettings{
		Host:              "localhost",
		Port:              32400,
		TLS:               false,
		PlexUser:          "TestUser",
		AnimeLibraries:    []string{"Anime"},
		PlexClientEnabled: false,
		Token:             []byte("test-token"),
		TokenIV:           tokenIV,
	}
	_, err := plexSettingsService.Store(ctx, plexSettings)
	require.NoError(t, err)

	// Setup: Store anime in database for mapping lookup
	animeRepo := database.NewAnimeRepo(log, db)
	anime := []*domain.Anime{
		{
			MALId:     1575,
			TVDBId:    81797,
			MainTitle: "One Piece",
		},
	}
	err = animeRepo.StoreMultiple(anime)
	require.NoError(t, err)

	t.Run("plex webhook triggers event flow", func(t *testing.T) {
		// Track events
		eventsReceived := make(map[string]bool)
		var plexProcessedEvent *domain.PlexProcessedSuccessEvent
		var animeUpdateSuccessEvent *domain.AnimeUpdateSuccessEvent
		var animeUpdateFailedEvent *domain.AnimeUpdateFailedEvent

		// Subscribe to events
		err := (*bus).Subscribe(domain.EventPlexProcessedSuccess, func(event *domain.PlexProcessedSuccessEvent) {
			eventsReceived[domain.EventPlexProcessedSuccess] = true
			plexProcessedEvent = event
		})
		require.NoError(t, err)

		err = (*bus).Subscribe(domain.EventPlexProcessedError, func(event *domain.PlexProcessedErrorEvent) {
			eventsReceived[domain.EventPlexProcessedError] = true
		})
		require.NoError(t, err)

		err = (*bus).Subscribe(domain.EventAnimeUpdateSuccess, func(event *domain.AnimeUpdateSuccessEvent) {
			eventsReceived[domain.EventAnimeUpdateSuccess] = true
			animeUpdateSuccessEvent = event
		})
		require.NoError(t, err)

		err = (*bus).Subscribe(domain.EventAnimeUpdateFailed, func(event *domain.AnimeUpdateFailedEvent) {
			eventsReceived[domain.EventAnimeUpdateFailed] = true
			animeUpdateFailedEvent = event
		})
		require.NoError(t, err)

		// Create Plex webhook payload
		payload := testdata.RawPlexWebhookHAMAEpisode()

		// Create multipart form request
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		err = writer.WriteField("payload", payload)
		require.NoError(t, err)
		writer.Close()

		// Create API key for authentication
		key := &domain.APIKey{
			Name:   "Test API Key",
			Scopes: []string{"read", "write"},
		}
		err = apiService.Store(ctx, key)
		require.NoError(t, err)
		require.NotEmpty(t, key.Key, "API key should be generated")

		// Populate cache
		_, err = apiService.List(ctx)
		require.NoError(t, err)

		// Make HTTP request with API key authentication
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/plex", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Shinkro-Api-Key", key.Key)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify response
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Wait for async event processing
		time.Sleep(500 * time.Millisecond)

		// Verify Plex processed event was published
		assert.True(t,
			eventsReceived[domain.EventPlexProcessedSuccess] || eventsReceived[domain.EventPlexProcessedError],
			"Plex processed event should be published")

		// If processing succeeded, verify anime update flow
		if eventsReceived[domain.EventPlexProcessedSuccess] {
			require.NotNil(t, plexProcessedEvent, "Plex processed event should contain data")
			require.NotNil(t, plexProcessedEvent.AnimeUpdate, "Plex processed event should contain anime update")

			// Verify anime update event was published (success or failure)
			// Since we don't have MAL auth configured, it will likely fail
			assert.True(t,
				eventsReceived[domain.EventAnimeUpdateSuccess] || eventsReceived[domain.EventAnimeUpdateFailed],
				"Anime update event should be published")

			// Verify anime update was stored in database
			animeUpdateRepo := database.NewAnimeUpdateRepo(log, db)
			if animeUpdateSuccessEvent != nil {
				update, err := animeUpdateRepo.GetByPlexID(ctx, animeUpdateSuccessEvent.PlexID)
				require.NoError(t, err)
				require.NotNil(t, update)
				assert.Equal(t, domain.AnimeUpdateStatusSuccess, update.Status)
			} else if animeUpdateFailedEvent != nil {
				update, err := animeUpdateRepo.GetByPlexID(ctx, animeUpdateFailedEvent.AnimeUpdate.PlexId)
				require.NoError(t, err)
				require.NotNil(t, update)
				assert.Equal(t, domain.AnimeUpdateStatusFailed, update.Status)
				assert.NotEmpty(t, update.ErrorType)
			}
		}

		// Verify Plex payload was stored
		plexRepo := database.NewPlexRepo(log, db)
		plexPayloads, err := plexRepo.GetRecent(ctx, 1)
		require.NoError(t, err)
		require.Greater(t, len(plexPayloads), 0)
	})
}
