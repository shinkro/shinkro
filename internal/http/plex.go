package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/hlog"
	"github.com/shinkro/shinkro/internal/domain"
)

type plexService interface {
	Store(ctx context.Context, plex *domain.Plex) error
	Get(ctx context.Context, req *domain.GetPlexRequest) (*domain.Plex, error)
	ProcessPlex(ctx context.Context, plex *domain.Plex) error
	GetPlexSettings(ctx context.Context) (*domain.PlexSettings, error)
	CheckPlex(ctx context.Context, plex *domain.Plex, ps *domain.PlexSettings) error
	CountScrobbleEvents(ctx context.Context) (int, error)
	CountRateEvents(ctx context.Context) (int, error)
	GetPlexHistory(ctx context.Context, limit int) ([]domain.PlexHistoryItem, error)
	FindAllWithFilters(ctx context.Context, params domain.PlexPayloadQueryParams) (*domain.FindPlexPayloadsResponse, error)
	Delete(ctx context.Context, req *domain.DeletePlexRequest) error
}

type plexHandler struct {
	encoder encoder
	service plexService
}

func newPlexHandler(encoder encoder, service plexService) *plexHandler {
	return &plexHandler{
		encoder: encoder,
		service: service,
	}
}

func (h plexHandler) Routes(r chi.Router) {
	r.Get("/", h.getPlex)
	r.Get("/count", h.getCounts)
	r.With(middleware.AllowContentType("application/json", "multipart/form-data"), parsePlexPayload).Post("/", h.postPlex)
	r.Get("/history", h.getHistory)
	r.Get("/payloads", h.getPayloads)
	r.Delete("/payloads/{id}", h.deletePayload)
}

func (h plexHandler) getPlex(w http.ResponseWriter, r *http.Request) {
	idP := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idP)
	if err != nil && idP != "" {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST_PARAMS",
			"message": "id parameter is invalid",
		})
		return
	}

	plex, err := h.service.Get(r.Context(), &domain.GetPlexRequest{Id: id})
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}

	ret := struct {
		Data *domain.Plex `json:"data"`
	}{
		Data: plex,
	}

	h.encoder.StatusResponse(w, http.StatusOK, ret)

}

func (h plexHandler) postPlex(w http.ResponseWriter, r *http.Request) {
	// Safely extract plex payload from context
	plexValue := r.Context().Value(domain.PlexPayload)
	if plexValue == nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": "payload not found in request context",
		})
		return
	}

	plex, ok := plexValue.(*domain.Plex)
	if !ok || plex == nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": "invalid payload type in request context",
		})
		return
	}

	plexSettings, err := h.service.GetPlexSettings(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": err.Error(),
		})
		return
	}

	err = h.service.CheckPlex(r.Context(), plex, plexSettings)
	if err != nil {
		log := hlog.FromRequest(r)
		log.Debug().Err(err).Msg("Plex payload not sent for processing")
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": err.Error(),
		})
		return
	}

	err = h.service.Store(r.Context(), plex)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": err.Error(),
		})
		return
	}

	err = h.service.ProcessPlex(r.Context(), plex)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}
	// if !h.service.CheckPlex(plex) {
	// 	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
	// 		"code":    "OK",
	// 		"message": "Check Plex false",
	// 	})
	// 	return
	// }

	h.encoder.StatusCreated(w)
}

func (h plexHandler) getCounts(w http.ResponseWriter, r *http.Request) {
	countScrobble, err := h.service.CountScrobbleEvents(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}
	countRate, err := h.service.CountRateEvents(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}
	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"countScrobble": countScrobble,
		"countRate":     countRate,
	})
}

func (h plexHandler) getHistory(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	items, err := h.service.GetPlexHistory(r.Context(), limit)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, items)
}

func (h plexHandler) getPayloads(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter
	limit := uint64(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.ParseUint(l, 10, 64); err == nil && n > 0 {
			limit = n
		}
	}

	// Parse offset parameter
	offset := uint64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.ParseUint(o, 10, 64); err == nil {
			offset = n
		}
	}

	// Parse search parameter
	search := r.URL.Query().Get("q")

	// Parse filters
	eventStr := r.URL.Query().Get("event")
	sourceStr := r.URL.Query().Get("source")
	statusStr := r.URL.Query().Get("status")

	var event domain.PlexEvent
	if eventStr != "" {
		event = domain.PlexEvent(eventStr)
	}

	var source domain.PlexPayloadSource
	if sourceStr != "" {
		source = domain.PlexPayloadSource(sourceStr)
	}

	var status *bool
	if statusStr != "" {
		switch statusStr {
		case "success":
			val := true
			status = &val
		case "failed":
			val := false
			status = &val
		}
	}

	params := domain.PlexPayloadQueryParams{
		Limit:  limit,
		Offset: offset,
		Search: search,
		Filters: struct {
			Event  domain.PlexEvent
			Source domain.PlexPayloadSource
			Status *bool
		}{
			Event:  event,
			Source: source,
			Status: status,
		},
	}

	resp, err := h.service.FindAllWithFilters(r.Context(), params)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, resp)
}

func (h plexHandler) deletePayload(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "BAD_REQUEST",
			"message": "invalid plex ID",
		})
		return
	}

	err = h.service.Delete(r.Context(), &domain.DeletePlexRequest{Id: id})
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"message": "plex payload deleted successfully",
	})
}
