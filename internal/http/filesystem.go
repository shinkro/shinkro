package http

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/shinkro/shinkro/internal/domain"
	"net/http"
	"path/filepath"
)

type filesystemService interface {
	ListDir(ctx context.Context, path string) ([]domain.FileEntry, error)
	ListLogs(ctx context.Context) ([]domain.LogFile, error)
	DownloadLogFile(ctx context.Context, filename string) ([]byte, error)
}

type filesystemHandler struct {
	encoder encoder
	service filesystemService
}

func newFilesystemHandler(encoder encoder, service filesystemService) *filesystemHandler {
	return &filesystemHandler{
		encoder: encoder,
		service: service,
	}
}

func (h filesystemHandler) Routes(r chi.Router) {
	r.Get("/", h.getFileSystem)
	r.Route("/logs", func(r chi.Router) {
		r.Get("/", h.getLogs)
		r.Get("/{filename}", h.downloadLogFile)
	})
}

func (h filesystemHandler) getFileSystem(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	entries, err := h.service.ListDir(r.Context(), path)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, entries)
}

func (h filesystemHandler) getLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.ListLogs(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, logs)
}

func (h filesystemHandler) downloadLogFile(w http.ResponseWriter, r *http.Request) {
	filePath := chi.URLParam(r, "filename")
	if filePath == "" {
		h.encoder.Error(w, fmt.Errorf("filePath is required"))
		return
	}

	content, err := h.service.DownloadLogFile(r.Context(), filePath)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))

	// Write the file content to the response
	_, err = w.Write(content)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}
}
