package http

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/shinkro/shinkro/internal/domain"
	"net/http"
)

type mappingService interface {
	Store(ctx context.Context, m *domain.MapSettings) error
	Get(ctx context.Context) (*domain.MapSettings, error)
	ValidateMap(ctx context.Context, yamlPath string, isTVDB bool) error
}

type mappingHandler struct {
	service mappingService
	encoder encoder
}

type validationJson struct {
	YamlPath string `json:"yamlPath"`
	IsTVDB   bool   `json:"isTVDB"`
}

func newMappingHandler(encoder encoder, service mappingService) *mappingHandler {
	return &mappingHandler{
		service: service,
		encoder: encoder,
	}
}

func (h mappingHandler) Routes(r chi.Router) {
	r.Get("/", h.get)
	r.Post("/", h.store)
	r.Post("/validate", h.validateMap)
}

func (h mappingHandler) get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Get(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, settings)
}

func (h mappingHandler) store(w http.ResponseWriter, r *http.Request) {
	var data domain.MapSettings
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.Error(w, err)
		return
	}

	if err := h.service.Store(r.Context(), &data); err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "mapping settings updated")
}

func (h mappingHandler) validateMap(w http.ResponseWriter, r *http.Request) {
	var data validationJson
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.StatusResponseMessage(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.ValidateMap(r.Context(), data.YamlPath, data.IsTVDB); err != nil {
		h.encoder.StatusResponseMessage(w, http.StatusNotAcceptable, err.Error())
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "mapping validated")
}
