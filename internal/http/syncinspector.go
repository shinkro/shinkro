package http

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/shinkro/shinkro/internal/syncinspector"
)

func (s Server) WithSyncInspector(inspector *syncinspector.Service) Server {
	s.syncInspector = inspector
	return s
}
func syncJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func syncError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	e := &syncinspector.Error{Code: "INTERNAL_ERROR", Message: "The sync operation could not be completed."}
	if err == sql.ErrNoRows {
		status = http.StatusNotFound
		e = &syncinspector.Error{Code: "NOT_FOUND", Message: "The requested event, preview, or rule no longer exists."}
	} else if typed, ok := err.(*syncinspector.Error); ok {
		e = typed
		status = http.StatusBadRequest
		switch e.Code {
		case "WRITES_DISABLED":
			status = http.StatusForbidden
		case "JOB_IN_PROGRESS", "STALE_PREVIEW", "STALE_RULE_PREVIEW", "RULE_CONFLICT", "REVISION_CONFLICT", "UNCERTAIN_WRITE":
			status = http.StatusConflict
		case "SOURCE_UNAVAILABLE":
			status = http.StatusBadGateway
		}
	}
	syncJSON(w, status, e)
}
func (s Server) syncOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := r.Context().Value(sessionkey).(*sessions.Session)
		if !ok || session == nil || session.Values["authenticated"] != true {
			syncJSON(w, http.StatusForbidden, &syncinspector.Error{Code: "OPERATOR_SESSION_REQUIRED", Message: "Sign in with your Shinkro account to use Sync Inspector."})
			return
		}
		actorID, _ := session.Values["user_id"].(int)
		if r.Method != "GET" && r.Method != "HEAD" && actorID < 1 {
			syncJSON(w, http.StatusForbidden, &syncinspector.Error{Code: "SIGN_IN_REQUIRED", Message: "Sign in again to identify the operator performing this action."})
			return
		}
		if actorID > 0 {
			r = r.WithContext(syncinspector.WithActor(r.Context(), actorID))
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				syncJSON(w, http.StatusForbidden, &syncinspector.Error{Code: "INVALID_ORIGIN", Message: "This request must originate from this Shinkro instance."})
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" {

			token, _ := session.Values["sync_csrf"].(string)
			sent := r.Header.Get("X-Shinkro-CSRF")
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(sent)) != 1 {
				syncJSON(w, http.StatusForbidden, &syncinspector.Error{Code: "INVALID_CSRF", Message: "Refresh Sync Inspector before trying this action again."})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func syncDecode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		syncJSON(w, http.StatusBadRequest, &syncinspector.Error{Code: "INVALID_INPUT", Message: "The request body is invalid."})
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		syncJSON(w, http.StatusBadRequest, &syncinspector.Error{Code: "INVALID_INPUT", Message: "Send exactly one JSON object."})
		return false
	}
	return true
}
func (s Server) syncRoutes(r chi.Router) {
	r.Use(s.syncOperator)
	svc := s.syncInspector
	r.Get("/capabilities", func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(sessionkey).(*sessions.Session)
		token, _ := session.Values["sync_csrf"].(string)
		if token == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				syncError(w, err)
				return
			}
			token = hex.EncodeToString(b)
			session.Values["sync_csrf"] = token
			session.Options = userSessionOptions(r, s.config.Config.BaseUrl, 30*24*60*60)
			if err := session.Save(r, w); err != nil {
				syncError(w, err)
				return
			}
		}
		syncJSON(w, 200, map[string]interface{}{"writesEnabled": svc.WritesEnabled(), "csrfToken": token, "previewTTLSeconds": 300})
	})
	r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		data, count, err := svc.List(r.Context(), limit, offset, q.Get("search"), q.Get("status"), q.Get("source"), q.Get("event"))
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, map[string]interface{}{"data": data, "count": count})
	})
	r.Get("/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		key, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		d, err := svc.Detail(r.Context(), key)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, d)
	})
	r.Post("/events/{id}/preview", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Correction *syncinspector.Correction `json:"correction,omitempty"`
		}
		if !syncDecode(w, r, &req) {
			return
		}
		key, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		ctx, cancel := contextTimeout(r, 40*time.Second)
		defer cancel()
		p, err := svc.Preview(ctx, key, req.Correction)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, p)
	})
	r.Post("/events/{id}/replays", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PreviewID string `json:"previewId"`
		}
		if !syncDecode(w, r, &req) {
			return
		}
		key, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		ctx, cancel := contextTimeout(r, 40*time.Second)
		defer cancel()
		j, err := svc.Replay(ctx, key, req.PreviewID)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 202, j)
	})
	r.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		j, err := svc.Job(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, j)
	})
	r.Get("/mal/search", func(w http.ResponseWriter, r *http.Request) {
		data, err := svc.Search(r.Context(), r.URL.Query().Get("q"))
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, map[string]interface{}{"data": data})
	})
	r.Post("/sources/refresh", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextTimeout(r, 90*time.Second)
		defer cancel()
		if err := svc.Refresh(ctx); err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, map[string]interface{}{"refreshedAt": time.Now().UTC()})
	})
	r.Get("/overrides", func(w http.ResponseWriter, r *http.Request) {
		data, err := svc.AllOverrides(r.Context())
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, map[string]interface{}{"data": data})
	})
	r.Post("/overrides", func(w http.ResponseWriter, r *http.Request) {
		var req syncinspector.Override
		if !syncDecode(w, r, &req) {
			return
		}
		o, err := svc.SaveOverride(r.Context(), req)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 201, o)
	})
	r.Delete("/overrides/{id}", func(w http.ResponseWriter, r *http.Request) {
		rev, _ := strconv.Atoi(r.URL.Query().Get("revision"))
		if err := svc.DeleteOverrideRevision(r.Context(), chi.URLParam(r, "id"), rev); err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 204, nil)
	})
	r.Post("/overrides/preview", func(w http.ResponseWriter, r *http.Request) {
		var req syncinspector.Override
		if !syncDecode(w, r, &req) {
			return
		}
		impact, err := svc.OverrideImpact(r.Context(), req)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, impact)
	})
	r.Put("/overrides/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req syncinspector.Override
		if !syncDecode(w, r, &req) {
			return
		}
		req.ID = chi.URLParam(r, "id")
		o, err := svc.UpdateOverride(r.Context(), req)
		if err != nil {
			syncError(w, err)
			return
		}
		syncJSON(w, 200, o)
	})
}

func contextTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}
