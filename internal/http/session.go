package http

import (
	"github.com/gorilla/sessions"
	"net/http"
)

// Cookie options are request-local: gorilla does not serialize options into the
// cookie, and defaults must not overwrite secure/HttpOnly flags during renewal.
func userSessionOptions(r *http.Request, base string, maxAge int) *sessions.Options {
	if base == "" {
		base = "/"
	}
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteStrictMode
	}
	return &sessions.Options{Path: base, MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: sameSite}
}
