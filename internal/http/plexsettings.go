package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/shinkro/shinkro/pkg/plex"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

type plexsettingsService interface {
	Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error)
	Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error)
	Get(ctx context.Context) (*domain.PlexSettings, error)
	Delete(ctx context.Context) error
	GetClient(ctx context.Context, ps *domain.PlexSettings) (*plex.Client, error)
}

type plexsettingsHandler struct {
	encoder encoder
	service plexsettingsService
}

func newPlexsettingsHandler(encoder encoder, service plexsettingsService) *plexsettingsHandler {
	return &plexsettingsHandler{
		encoder: encoder,
		service: service,
	}
}

func (h plexsettingsHandler) Routes(r chi.Router) {
	r.Get("/", h.getPlexSettings)
	r.Put("/", h.putPlexSettings)
	r.Delete("/", h.deletePlexSettings)
	r.Get("/testToken", h.testToken)
	r.Post("/test", h.test)
	r.Post("/oauth", h.startOAuth)
	r.Get("/oauth", h.pollOAuth)
	r.Post("/servers", h.getServers)
	r.Post("/libraries", h.getLibraries)
}

func (h plexsettingsHandler) getPlexSettings(w http.ResponseWriter, r *http.Request) {
	ps, err := h.service.Get(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		h.encoder.NoContent(w)
		return
	}
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "PLEX_SETTINGS_ERROR",
			"message": err.Error(),
		})
		return
	}
	h.encoder.StatusResponse(w, http.StatusOK, ps)
}

func (h plexsettingsHandler) deletePlexSettings(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.NoContent(w)
}

func (h plexsettingsHandler) putPlexSettings(w http.ResponseWriter, r *http.Request) {
	var data domain.PlexSettings
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.Error(w, err)
		return
	}

	ps, err := h.service.Update(r.Context(), data)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, ps)
}

func (h plexsettingsHandler) test(w http.ResponseWriter, r *http.Request) {
	var ps domain.PlexSettings
	if err := json.NewDecoder(r.Body).Decode(&ps); err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "PLEX_SETTINGS_ERROR",
			"message": err.Error(),
		})
		return
	}

	pc, err := h.service.GetClient(r.Context(), &ps)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "PLEX_CLIENT_ERROR",
			"message": err.Error(),
		})
		return
	}

	err = pc.TestConnection(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "PLEX_CONNECTION_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "Plex connection successful")
}

func (h plexsettingsHandler) testToken(w http.ResponseWriter, r *http.Request) {
	ps, err := h.service.Get(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusUnauthorized, map[string]interface{}{
			"code":    "PLEX_TOKEN_NOT_FOUND",
			"message": err.Error(),
		})
		return
	}

	pc, err := h.service.GetClient(r.Context(), ps)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]interface{}{
			"code":    "PLEX_CLIENT_ERROR",
			"message": err.Error(),
		})
		return
	}

	err = pc.TestToken(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusUnauthorized, map[string]interface{}{
			"code":    "PLEX_TOKEN_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponseMessage(w, http.StatusOK, "Plex token is valid")
}

func (h plexsettingsHandler) startOAuth(w http.ResponseWriter, r *http.Request) {
	clientID := generateClientId()

	data := url.Values{
		"strong":                   {"true"},
		"X-Plex-Product":           {"shinkro"},
		"X-Plex-Client-Identifier": {clientID},
	}

	req, err := http.NewRequestWithContext(r.Context(), "POST", "https://plex.tv/api/v2/pins", strings.NewReader(data.Encode()))
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	client := &http.Client{Transport: sharedhttp.Transport}
	resp, err := client.Do(req)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusInternalServerError, map[string]string{
			"message": "Failed to initiate OAuth: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var pinResp struct {
		ID        int    `json:"id"`
		Code      string `json:"code"`
		ExpiresIn int    `json:"expires_in"`
		ClientID  string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		h.encoder.Error(w, err)
		return
	}

	authURL := fmt.Sprintf(
		"https://app.plex.tv/auth#?clientID=%s&code=%s&context%%5Bdevice%%5D%%5Bproduct%%5D=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(pinResp.Code),
		url.QueryEscape("shinkro"),
	)

	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"pin_id":    pinResp.ID,
		"code":      pinResp.Code,
		"client_id": clientID,
		"auth_url":  authURL,
	})
}

func (h plexsettingsHandler) pollOAuth(w http.ResponseWriter, r *http.Request) {
	pinID := r.URL.Query().Get("pin_id")
	clientID := r.URL.Query().Get("client_id")
	code := r.URL.Query().Get("code")

	if pinID == "" || clientID == "" || code == "" {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"message": "Missing pin_id, client_id, or code",
		})
		return
	}

	data := strings.NewReader(fmt.Sprintf(`code=%s&X-Plex-Client-Identifier=%s`, code, clientID))
	req, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("https://plex.tv/api/v2/pins/%v", pinID), data)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	client := &http.Client{Transport: sharedhttp.Transport}
	resp, err := client.Do(req)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadGateway, map[string]string{
			"message": "Polling failed: " + err.Error(),
		})
		return
	}

	type pollResp struct {
		AuthToken *string `json:"authToken"`
	}

	var tokenresp pollResp
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	if err := json.Unmarshal(body, &tokenresp); err != nil {
		h.encoder.Error(w, err)
		return
	}

	if tokenresp.AuthToken == nil {
		h.encoder.StatusResponse(w, http.StatusAccepted, map[string]string{
			"message": "waiting for auth",
		})
		return
	}

	var plexDetails struct {
		PlexUser string `json:"username"`
	}

	req, err = http.NewRequestWithContext(r.Context(), "GET", "https://plex.tv/api/v2/user", nil)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", *tokenresp.AuthToken)
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	resp, err = client.Do(req)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadGateway, map[string]string{
			"message": "Failed to get user details: " + err.Error(),
		})
		return
	}

	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	err = json.Unmarshal(b, &plexDetails)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	tokenIV, err := generateRandomIV()
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	var p = domain.PlexSettings{
		ClientID: clientID,
		PlexUser: plexDetails.PlexUser,
		Token:    []byte(*tokenresp.AuthToken),
		TokenIV:  tokenIV,
	}

	_, err = h.service.Store(r.Context(), p)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, map[string]interface{}{
		"plex_user": plexDetails.PlexUser,
		"client_id": clientID,
	})
}

func (h plexsettingsHandler) getServers(w http.ResponseWriter, r *http.Request) {
	var ps domain.PlexSettings
	if err := json.NewDecoder(r.Body).Decode(&ps); err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "PLEX_SETTINGS_ERROR",
			"message": err.Error(),
		})
		return
	}

	pc, err := h.service.GetClient(r.Context(), &ps)
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	servers, err := pc.GetServerList(r.Context())
	if err != nil {
		h.encoder.Error(w, err)
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, servers)
}

func (h plexsettingsHandler) getLibraries(w http.ResponseWriter, r *http.Request) {
	var ps domain.PlexSettings
	if err := json.NewDecoder(r.Body).Decode(&ps); err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "PLEX_SETTINGS_ERROR",
			"message": err.Error(),
		})
		return
	}

	pc, err := h.service.GetClient(r.Context(), &ps)
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "PLEX_CLIENT_ERROR",
			"message": err.Error(),
		})
		return
	}

	libraries, err := pc.GetLibraries(r.Context())
	if err != nil {
		h.encoder.StatusResponse(w, http.StatusBadRequest, map[string]string{
			"code":    "PLEX_LIBRARIES_ERROR",
			"message": err.Error(),
		})
		return
	}

	h.encoder.StatusResponse(w, http.StatusOK, libraries)
}
