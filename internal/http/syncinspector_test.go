package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/config"
	"github.com/shinkro/shinkro/internal/database"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/syncinspector"
	"github.com/stretchr/testify/require"
)

type syncAPIKeys struct{}

func (syncAPIKeys) List(context.Context) ([]domain.APIKey, error) { return nil, nil }
func (syncAPIKeys) Store(context.Context, *domain.APIKey) error   { return nil }
func (syncAPIKeys) Delete(context.Context, string) error          { return nil }
func (syncAPIKeys) ValidateAPIKey(context.Context, string) bool   { return true }
func TestSyncInspectorOperatorAndCSRF(t *testing.T) {
	s := Server{log: zerolog.Nop(), config: &config.AppConfig{Config: &domain.Config{BaseUrl: "/"}}, cookieStore: sessions.NewCookieStore([]byte("test-only-session-signing-key")), apiService: syncAPIKeys{}, syncInspector: syncinspector.New(nil, nil, nil, false, nil, nil, nil)}
	router := chi.NewRouter()
	router.Use(s.IsAuthenticated)
	router.Route("/sync", s.syncRoutes)
	// A valid webhook key must never grant access to operator routes.
	r := httptest.NewRequest("GET", "http://shinkro.test/sync/capabilities", nil)
	r.Header.Set("Shinkro-Api-Key", "valid-webhook-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "OPERATOR_SESSION_REQUIRED")
	cookie := func(operator bool) *http.Cookie {
		r := httptest.NewRequest("GET", "http://shinkro.test/", nil)
		w := httptest.NewRecorder()
		session, _ := s.cookieStore.Get(r, "user_session")
		session.Values["authenticated"] = true
		session.Values["created"] = time.Now().Unix()
		if operator {
			session.Values["user_id"] = 1
		}
		session.Options.Path = "/"
		session.Options.MaxAge = 3600
		require.NoError(t, session.Save(r, w))
		return w.Result().Cookies()[0]
	}
	r = httptest.NewRequest("GET", "http://shinkro.test/sync/capabilities", nil)
	r.AddCookie(cookie(true))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, 200, w.Code)
	var caps struct {
		CSRFToken     string `json:"csrfToken"`
		WritesEnabled bool   `json:"writesEnabled"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &caps))
	require.NotEmpty(t, caps.CSRFToken)
	require.False(t, caps.WritesEnabled)
	signedCookie := w.Result().Cookies()[0]
	for _, tc := range []struct {
		name, token, origin string
		operator            bool
		code                string
	}{{"missing token", "", "", true, "INVALID_CSRF"}, {"wrong token", "wrong", "", true, "INVALID_CSRF"}, {"foreign origin", caps.CSRFToken, "https://attacker.test", true, "INVALID_ORIGIN"}, {"old unbound session", caps.CSRFToken, "", false, "SIGN_IN_REQUIRED"}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://shinkro.test/sync/sources/refresh", bytes.NewBufferString("{}"))
			if tc.operator {
				r.AddCookie(signedCookie)
			} else {
				r.AddCookie(cookie(false))
			}
			r.Header.Set("X-Shinkro-CSRF", tc.token)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Body.String(), tc.code)
		})
	}
	// Authorized mutation reaches the server-enforced no-write policy.
	r = httptest.NewRequest("POST", "http://shinkro.test/sync/events/1/replays", bytes.NewBufferString(`{"previewId":"x"}`))
	r.AddCookie(signedCookie)
	r.Header.Set("X-Shinkro-CSRF", caps.CSRFToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), "WRITES_DISABLED")
}
func TestSyncDecodeRejectsUnknownTrailingAndOversizedJSON(t *testing.T) {
	for _, body := range []string{`{"previewId":"x"} {}`, `{"previewId":"x","unknown":1}`, `not json`, string(bytes.Repeat([]byte(" "), 65<<10)) + `{}`} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		var req struct {
			PreviewID string `json:"previewId"`
		}
		require.False(t, syncDecode(w, r, &req))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

type syncAuth struct{}

func (syncAuth) GetUserCount(context.Context) (int, error) { return 1, nil }
func (syncAuth) Login(context.Context, string, string) (*domain.User, error) {
	return &domain.User{ID: 1, Username: "reviewer"}, nil
}
func (syncAuth) CreateUser(context.Context, domain.CreateUserRequest) error { return nil }
func (syncAuth) UpdateUser(context.Context, domain.UpdateUserRequest) error { return nil }
func TestSyncInspectorRealCookieJarRoundTrip(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "HTTPS"}[secure], func(t *testing.T) {
			log := zerolog.Nop()
			db := database.NewDB(t.TempDir(), &log)
			defer db.Close()
			require.NoError(t, db.Migrate())
			cfg := &config.AppConfig{Config: &domain.Config{BaseUrl: "/", SessionSecret: "test-only-cookie-signing-key"}}
			s := NewServer(log, cfg, db, "test", "test", "test", nil, nil, nil, syncAPIKeys{}, syncAuth{}, nil, nil, nil, nil, nil).WithSyncInspector(syncinspector.New(db.SQL(), nil, nil, false, nil, nil, nil))
			var ts *httptest.Server
			if secure {
				ts = httptest.NewTLSServer(s.Handler())
			} else {
				ts = httptest.NewServer(s.Handler())
			}
			defer ts.Close()
			client := ts.Client()
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			client.Jar = jar
			resp, err := client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewBufferString(`{"username":"reviewer","password":"private-test-value"}`))
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, resp.StatusCode)
			resp.Body.Close()
			require.Len(t, resp.Cookies(), 1)
			require.True(t, resp.Cookies()[0].HttpOnly)
			require.Equal(t, secure, resp.Cookies()[0].Secure)
			resp, err = client.Get(ts.URL + "/api/sync/capabilities")
			require.NoError(t, err)
			require.Equal(t, 200, resp.StatusCode)
			var caps struct {
				CSRFToken string `json:"csrfToken"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&caps))
			resp.Body.Close()
			require.NotEmpty(t, caps.CSRFToken)
			require.Len(t, resp.Cookies(), 1)
			require.True(t, resp.Cookies()[0].HttpOnly)
			require.Equal(t, secure, resp.Cookies()[0].Secure)
			resp, err = client.Get(ts.URL + "/api/sync/events")
			require.NoError(t, err)
			require.Equal(t, 200, resp.StatusCode)
			resp.Body.Close()
			req, err := http.NewRequest("POST", ts.URL+"/api/sync/events/1/replays", bytes.NewBufferString(`{"previewId":"x"}`))
			require.NoError(t, err)
			req.Header.Set("X-Shinkro-CSRF", caps.CSRFToken)
			resp, err = client.Do(req)
			require.NoError(t, err)
			require.Equal(t, 403, resp.StatusCode)
			var body syncinspector.Error
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			resp.Body.Close()
			require.Equal(t, "WRITES_DISABLED", body.Code)
		})
	}
}
