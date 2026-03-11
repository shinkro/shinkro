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
	// AniList sends the callback as a GET request with ?code=...&state=...
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
	if err := h.service.Delete(r.Context()); err != nil {
		h.encoder.Error(w, err)
		return
	}
	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth deleted")
}

func (h anilistAuthHandler) startOauth(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("clientID")
	clientSecret := r.URL.Query().Get("clientSecret")
	if clientID == "" || clientSecret == "" {
		h.encoder.Error(w, errors.New("clientID or clientSecret is empty"))
		return
	}

	tokenIV, err := generateRandomIV()
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	redirectURL := fmt.Sprintf("http://localhost:%d/api/anilistauth/callback", h.appPort)
	aa := domain.NewAnilistAuth(clientID, clientSecret, redirectURL, nil, tokenIV)

	// Generate state and store it in the DB alongside the credentials.
	// This avoids cookie issues since AniList callback is a GET to the backend
	// from a different browser context.
	state, err := generateState(64)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}
	aa.OAuthState = state

	if err = h.service.Store(r.Context(), aa); err != nil {
		h.encoder.Error(w, err)
		return
	}

	authCodeURL := aa.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"url": authCodeURL,
	})
}

// callback handles the GET redirect from AniList after user authorization.
// State is validated against the value stored in the DB (not a cookie),
// because AniList redirects the browser directly to this endpoint and
// browser cookies are unreliable across origins/ports.
func (h anilistAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	incomingState := r.URL.Query().Get("state")

	if code == "" || incomingState == "" {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "ANILIST_AUTH_ERROR",
			"message": "code or state is empty",
		})
		return
	}

	// Load credentials from DB and validate state
	aa, err := h.service.GetDecrypted(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	if aa.OAuthState == "" {
		h.encoder.StatusResponse(w, http.StatusRequestTimeout, map[string]string{
			"code":    "ANILIST_AUTH_TIMEOUT",
			"message": "OAuth state not found, request may have expired",
		})
		return
	}

	if aa.OAuthState != incomingState {
		h.encoder.Error(w, errors.New("state does not match"))
		return
	}

	// Exchange code for token
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

	// Save token and clear the state
	aa.AccessToken = t
	aa.OAuthState = ""
	if err = h.service.Store(r.Context(), aa); err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth success")
}

func (h anilistAuthHandler) test(w http.ResponseWriter, r *http.Request) {
	if _, err := h.service.GetAccessToken(r.Context()); err != nil {
		h.encoder.Error(w, err)
		return
	}
	h.encoder.StatusResponseMessage(w, http.StatusOK, "anilist auth test success")
}
