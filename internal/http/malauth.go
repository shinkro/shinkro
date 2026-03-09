package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/nstratos/go-myanimelist/mal"
	"github.com/pkg/errors"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/oauth2"
	"net/http"
)

type malauthService interface {
	Store(ctx context.Context, ma *domain.MalAuth) error
	Get(ctx context.Context) (*domain.MalAuth, error)
	Delete(ctx context.Context) error
	GetMalClient(ctx context.Context) (*mal.Client, error)
	GetDecrypted(ctx context.Context) (*domain.MalAuth, error)
}

type maConfig struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

type malauthHandler struct {
	cookieStore *sessions.CookieStore
	encoder     encoder
	service     malauthService
}

func newmalauthHandler(encoder encoder, service malauthService, cookieStore *sessions.CookieStore) *malauthHandler {
	return &malauthHandler{
		cookieStore: cookieStore,
		encoder:     encoder,
		service:     service,
	}
}

func (h malauthHandler) Routes(r chi.Router) {
	r.Get("/test", h.test)
	r.Get("/", h.get)
	r.Post("/", h.startOauth)
	r.Delete("/", h.delete)
	r.Post("/callback", h.callback)
}

func (h malauthHandler) get(w http.ResponseWriter, r *http.Request) {
	ma, err := h.service.Get(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		h.encoder.NoContent(w)
		return
	}

	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "MAL_AUTH_ERROR",
			"message": err.Error(),
		})
		return
	}

	resp := maConfig{
		ClientID:     ma.Config.ClientID,
		ClientSecret: ma.Config.ClientSecret,
	}

	h.encoder.StatusResponse(w, http.StatusOK, resp)
}

func (h malauthHandler) delete(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "mal auth deleted")
}

func (h malauthHandler) startOauth(w http.ResponseWriter, r *http.Request) {
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

	ma := domain.NewMalAuth(clientID, clientSecret, nil, tokenIV)
	verifier, challenge, err := generatePKCE(128)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	state, err := generateState(64)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	s, _ := h.cookieStore.Get(r, "mal_oauth_session")
	s.Values["state"] = state
	s.Values["verifier"] = verifier
	s.Options = &sessions.Options{
		MaxAge: 600,
	}

	err = s.Save(r, w)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	codeChallenge := oauth2.SetAuthURLParam("code_challenge", challenge)
	responseType := oauth2.SetAuthURLParam("response_type", "code")
	authCodeUrl := ma.Config.AuthCodeURL(state, codeChallenge, responseType)

	err = h.service.Store(r.Context(), ma)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"url": authCodeUrl,
	})
}

func (h malauthHandler) callback(w http.ResponseWriter, r *http.Request) {
	s, _ := h.cookieStore.Get(r, "mal_oauth_session")
	state, _ := s.Values["state"].(string)
	verifier, _ := s.Values["verifier"].(string)
	s.Options.MaxAge = -1
	_ = s.Save(r, w)

	code := r.URL.Query().Get("code")
	newState := r.URL.Query().Get("state")
	if code == "" || newState == "" {
		err := errors.New("code or state is empty")

		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "MALAUTH_ERROR",
			"message": err.Error(),
		})
		return
	}

	if state == "" || verifier == "" {
		err := errors.New("state or verifier is empty, request timed out")
		h.encoder.StatusResponse(w, http.StatusRequestTimeout, map[string]interface{}{
			"code":    "MALAUTH_TIMEOUT",
			"message": err.Error(),
		})
		return
	}

	if newState != state {
		err := errors.New("state does not match")
		h.encoder.Error(w, err)
		return
	}

	ma, err := h.service.GetDecrypted(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	grantType := oauth2.SetAuthURLParam("grant_type", "authorization_code")
	codeVerify := oauth2.SetAuthURLParam("code_verifier", verifier)
	token, err := ma.Config.Exchange(r.Context(), code, grantType, codeVerify)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	t, err := json.Marshal(token)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	ma.AccessToken = t
	err = h.service.Store(r.Context(), ma)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "mal auth success")
}

func (h malauthHandler) test(w http.ResponseWriter, r *http.Request) {

	c, err := h.service.GetMalClient(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}
	_, _, err = c.User.MyInfo(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "mal auth test success")

}
