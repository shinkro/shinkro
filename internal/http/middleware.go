package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/hlog"
	"github.com/shinkro/shinkro/internal/domain"
)

func (s Server) IsAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.Header.Get("Shinkro-Api-Key"); token != "" {
			// check header
			if !s.apiService.ValidateAPIKey(r.Context(), token) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

		} else if key := r.URL.Query().Get("apiKey"); key != "" {
			// check query param like ?apiKey=TOKEN
			if !s.apiService.ValidateAPIKey(r.Context(), key) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		} else {
			// check session
			session, err := s.cookieStore.Get(r, "user_session")
			if err != nil {
				s.log.Error().Err(err).Msgf("could not get session from cookieStore")
				session.Values["authenticated"] = false

				// MaxAge<0 means delete cookie immediately
				session.Options.MaxAge = -1
				session.Options.Path = s.config.Config.BaseUrl

				if err := session.Save(r, w); err != nil {
					s.log.Error().Err(err).Msgf("could not store session: %s", r.RemoteAddr)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			// Check if user is authenticated
			if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
				s.log.Warn().Msg("session not authenticated")

				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if created, ok := session.Values["created"].(int64); ok {
				// created is a unix timestamp MaxAge is in seconds
				maxAge := time.Duration(session.Options.MaxAge) * time.Second
				expires := time.Unix(created, 0).Add(maxAge)

				if time.Until(expires) <= 7*24*time.Hour { // 7 days
					s.log.Info().Msgf("Cookie is expiring in less than 7 days on %s - extending session", expires.Format("2006-01-02 15:04:05"))

					session.Values["created"] = time.Now().Unix()

					// Call session.Save as needed - since it writes a header (the Set-Cookie
					// header), making sure you call it before writing out a body is important.
					// https://github.com/gorilla/sessions/issues/178#issuecomment-447674812
					if err := session.Save(r, w); err != nil {
						s.log.Error().Err(err).Msgf("could not store session: %s", r.RemoteAddr)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}

			ctx := context.WithValue(r.Context(), sessionkey, session)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func parsePlexPayload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := hlog.FromRequest(r)

		// Wrap the response writer to track if a response was written
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		sourceType := contentType(r)
		msg := fmt.Sprintf("sourceType: %s", string(sourceType))
		log.Debug().Msg(msg)

		// Check if sourceType is supported (empty string means unsupported)
		if sourceType == "" {
			log.Error().Msg("unsupported content type")
			http.Error(ww, "unsupported content type", http.StatusBadRequest)
			return
		}

		payload, err := parsePayloadBySourceType(ww, r, sourceType)
		if err != nil {
			log.Debug().Err(err).Msg("could not parse payload")
			// parsePayloadBySourceType may have already written a response
			// (e.g., in handlePlexWebhook for empty payload or parse errors,
			// or in handleTautulli for read errors). Only send an error
			// response if one hasn't been written yet.
			if ww.Status() == 0 {
				http.Error(ww, "invalid payload", http.StatusBadRequest)
			}
			return
		}

		if payload == nil {
			log.Debug().Msg("payload is nil")
			// This shouldn't happen in normal flow, but handle it gracefully
			// Empty payload case is already handled in handlePlexWebhook with 204
			if ww.Status() == 0 {
				http.Error(ww, "invalid payload", http.StatusBadRequest)
			}
			return
		}

		ctx := context.WithValue(r.Context(), domain.PlexPayload, payload)
		log.Debug().Str("parsedPlexPayload", fmt.Sprintf("%+v", payload)).Msg("")
		next.ServeHTTP(ww, r.WithContext(ctx))
	})
}
