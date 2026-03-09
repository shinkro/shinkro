package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

type authService interface {
	GetUserCount(ctx context.Context) (int, error)
	Login(ctx context.Context, username, password string) (*domain.User, error)
	CreateUser(ctx context.Context, req domain.CreateUserRequest) error
	UpdateUser(ctx context.Context, req domain.UpdateUserRequest) error
}

type authHandler struct {
	log     zerolog.Logger
	encoder encoder
	config  *domain.Config
	service authService
	server  Server

	cookieStore *sessions.CookieStore
}

type session string

const sessionkey session = "session"

func newAuthHandler(encoder encoder, log zerolog.Logger, server Server, config *domain.Config, cookieStore *sessions.CookieStore, service authService) *authHandler {
	return &authHandler{
		log:         log,
		encoder:     encoder,
		config:      config,
		service:     service,
		cookieStore: cookieStore,
		server:      server,
	}
}

func (h authHandler) Routes(r chi.Router) {
	r.Post("/login", h.login)
	r.Post("/onboard", h.onboard)
	r.Get("/onboard", h.canOnboard)

	// Group for authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(h.server.IsAuthenticated)

		r.Post("/logout", h.logout)
		r.Get("/validate", h.validate)
		r.Patch("/user/{username}", h.updateUser)
	})
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	var data domain.User
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.StatusError(w, http.StatusBadRequest, errors.Wrap(err, "could not decode json"))
		return
	}

	if _, err := h.service.Login(r.Context(), data.Username, data.Password); err != nil {
		h.log.Error().Err(err).Msgf("Auth: Failed login attempt username: [%s] ip: %s", data.Username, r.RemoteAddr)
		h.encoder.StatusError(w, http.StatusForbidden, errors.New("could not login: bad credentials"))
		return
	}

	// create new session
	session, err := h.cookieStore.Get(r, "user_session")
	if err != nil {
		// clear any existing session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "user_session",
			Value:    "",
			Path:     h.config.BaseUrl,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
		})
		h.log.Error().Err(err).Msgf("Auth: Failed to create cookies with attempt username: [%s] ip: %s", data.Username, r.RemoteAddr)
		h.encoder.StatusError(w, http.StatusInternalServerError, errors.New("could not create cookies"))
		return
	}

	// Set user as authenticated
	session.Values["authenticated"] = true
	session.Values["created"] = time.Now().Unix()

	// Set cookie options
	session.Options.HttpOnly = true
	session.Options.Secure = false
	session.Options.SameSite = http.SameSiteLaxMode
	session.Options.Path = h.config.BaseUrl

	// shinkro does not support serving on TLS / https, so this is only available behind reverse proxy
	// if forwarded protocol is https then set cookie secure
	// SameSite Strict can only be set with a secure cookie. So we overwrite it here if possible.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie/SameSite
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		session.Options.Secure = true
		session.Options.SameSite = http.SameSiteStrictMode
	}

	if err := session.Save(r, w); err != nil {
		h.encoder.StatusError(w, http.StatusInternalServerError, errors.Wrap(err, "could not save session"))
		return
	}

	h.encoder.NoContent(w)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	// get session from context
	session, ok := r.Context().Value(sessionkey).(*sessions.Session)
	if !ok {
		h.encoder.StatusError(w, http.StatusInternalServerError, errors.New("could not get session from context"))
		return
	}

	if session != nil {
		session.Values["authenticated"] = false

		// MaxAge<0 means delete cookie immediately
		session.Options.MaxAge = -1

		session.Options.Path = h.config.BaseUrl

		if err := session.Save(r, w); err != nil {
			h.log.Error().Err(err).Msgf("could not store session: %s", r.RemoteAddr)
			h.encoder.StatusError(w, http.StatusInternalServerError, err)
			return
		}
	}

	h.encoder.StatusResponse(w, http.StatusNoContent, nil)
}

func (h authHandler) onboard(w http.ResponseWriter, r *http.Request) {
	if status, err := h.onboardEligible(r.Context()); err != nil {
		h.encoder.StatusError(w, status, err)
		return
	}

	var req domain.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.encoder.StatusError(w, http.StatusBadRequest, errors.Wrap(err, "could not decode json"))
		return
	}

	if err := h.service.CreateUser(r.Context(), req); err != nil {
		h.encoder.StatusError(w, http.StatusForbidden, err)
		return
	}

	// send response as ok
	h.encoder.StatusResponseMessage(w, http.StatusOK, "user successfully created")
}

func (h authHandler) canOnboard(w http.ResponseWriter, r *http.Request) {
	if status, err := h.onboardEligible(r.Context()); err != nil {
		h.encoder.StatusError(w, status, err)
		return
	}

	// send empty response as ok
	// (client can proceed with redirection to onboarding page)
	h.encoder.NoContent(w)
}

// onboardEligible checks if the onboarding process is eligible.
func (h authHandler) onboardEligible(ctx context.Context) (int, error) {
	userCount, err := h.service.GetUserCount(ctx)
	if err != nil {
		return http.StatusInternalServerError, errors.New("could not get user count")
	}

	if userCount > 0 {
		return http.StatusServiceUnavailable, errors.New("onboarding unavailable")
	}

	return http.StatusOK, nil
}

// validate sits behind the IsAuthenticated middleware which takes care of checking for a valid session
// If there is a valid session return OK, otherwise the middleware returns early with a 401
func (h authHandler) validate(w http.ResponseWriter, r *http.Request) {
	// Session is injected by IsAuthenticated middleware using the typed key `sessionkey`
	if v := r.Context().Value(sessionkey); v != nil {
		if session, ok := v.(*sessions.Session); !ok || session == nil {
			h.log.Error().Msg("session not authenticated")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}
	// send empty response as ok
	h.encoder.NoContent(w)
}

func (h authHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	var data domain.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.StatusError(w, http.StatusBadRequest, errors.Wrap(err, "could not decode json"))
		return
	}

	data.UsernameCurrent = chi.URLParam(r, "username")

	if err := h.service.UpdateUser(r.Context(), data); err != nil {
		h.encoder.StatusError(w, http.StatusBadRequest, err)
		return
	}

	// send response as ok
	h.encoder.StatusResponseMessage(w, http.StatusOK, "user successfully updated")
}
