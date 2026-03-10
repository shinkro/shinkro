package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/oauth2"
)

type anilistAuthService interface {
	Store(ctx context.Context, aa *domain.AnilistAuth) error
	Get(ctx context.Context) (*domain.AnilistAuth, error)
	Delete(ctx context.Context) error
	GetDecrypted(ctx context.Context) (*domain.AnilistAuth, error)
	GetAccessToken(ctx context.Context) (string, error)
}

type anilistConfig struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectURL"`
}

type anilistAuthHandler struct {
	cookieStore *sessions.CookieStore
	encoder     encoder
	service     anilistAuthService
	appPort     int
}

func newAnilistAuthHandler(encoder encoder, service anilistAuthService, cookieStore *sessions.CookieStore, appPort int) *anilistAuthHandler {
	return &anilistAuthHandler{
		cookieStore: cookieStore,
		encoder:     encoder,
		service:     service,
		appPort:     appPort,
	}
}

func (h anilistAuthHandler) Routes(r chi.Router) {
	r.Get("/test", h.test)
	r.Get("/", h.get)
	r.Post("/", h.startOauth)
	r.Delete("/", h.delete)
	// AniList sends the callback as a GET request with ?code=... in the URL
	r.Get("/callback", h.callback)
}

func (h anilistAuthHandler) get(w http.ResponseWriter, r *http.Request) {
	aa, err := h.service.Get(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		h.encoder.NoContent(w)
		return
	}

	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "ANILIST_AUTH_ERROR",
			"message": err.Error(),
		})
		return
	}

	resp := anilistConfig{
		ClientID:     aa.Config.ClientID,
		ClientSecret: aa.Config.ClientSecret,
		RedirectURL:  aa.Config.RedirectURL,
	}

	h.encoder.StatusResponse(w, http.StatusOK, resp)
}

func (h anilistAuthHandler) delete(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth deleted")
}

func (h anilistAuthHandler) startOauth(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("clientID")
	clientSecret := r.URL.Query().Get("clientSecret")
	if clientID == "" || clientSecret == "" {
		err := errors.New("clientID or clientSecret is empty")
		h.encoder.Error(w, err)
		return
	}

	tokenIV, err := generateRandomIV()
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	// Build the redirect URL based on the configured port
	redirectURL := fmt.Sprintf("http://localhost:%d/api/anilistauth/callback", h.appPort)

	aa := domain.NewAnilistAuth(clientID, clientSecret, redirectURL, nil, tokenIV)

	state, err := generateState(64)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	s, _ := h.cookieStore.Get(r, "anilist_oauth_session")
	s.Values["state"] = state
	s.Options = &sessions.Options{
		MaxAge: 600,
	}

	err = s.Save(r, w)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	authCodeURL := aa.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	err = h.service.Store(r.Context(), aa)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"url": authCodeURL,
	})
}

// callback handles the GET callback from AniList after user authorization.
// AniList redirects to: /api/anilistauth/callback?code=...&state=...
func (h anilistAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	s, _ := h.cookieStore.Get(r, "anilist_oauth_session")
	state, _ := s.Values["state"].(string)
	s.Options = &sessions.Options{MaxAge: -1}
	_ = s.Save(r, w)

	code := r.URL.Query().Get("code")
	newState := r.URL.Query().Get("state")

	if code == "" || newState == "" {
		err := errors.New("code or state is empty")
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "ANILIST_AUTH_ERROR",
			"message": err.Error(),
		})
		return
	}

	if state == "" {
		err := errors.New("state is empty, request timed out")
		h.encoder.StatusResponse(w, http.StatusRequestTimeout, map[string]interface{}{
			"code":    "ANILIST_AUTH_TIMEOUT",
			"message": err.Error(),
		})
		return
	}

	if newState != state {
		err := errors.New("state does not match")
		h.encoder.Error(w, err)
		return
	}

	aa, err := h.service.GetDecrypted(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	token, err := aa.Config.Exchange(r.Context(), code)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	t, err := json.Marshal(token)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	aa.AccessToken = t
	err = h.service.Store(r.Context(), aa)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth success")
}

func (h anilistAuthHandler) test(w http.ResponseWriter, r *http.Request) {
	_, err := h.service.GetAccessToken(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth test success")
}
