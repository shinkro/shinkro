package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/rs/zerolog/hlog"
	"github.com/shinkro/shinkro/internal/domain"
)

const InternalServerError string = "internal server error"

func contentType(r *http.Request) domain.PlexPayloadSource {
	contentType := r.Header.Get("Content-Type")
	var sourceType domain.PlexPayloadSource
	if strings.Contains(contentType, "multipart/form-data") {
		sourceType = domain.PlexWebhook
	} else if strings.Contains(contentType, "application/json") {
		sourceType = domain.TautulliWebhook
	}
	// If neither matches, sourceType remains empty string (zero value)
	// which indicates unsupported content type

	return sourceType
}

func readRequest(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	defer r.Body.Close()
	return string(b), nil
}

func parsePayloadBySourceType(w http.ResponseWriter, r *http.Request, sourceType domain.PlexPayloadSource) (*domain.Plex, error) {
	log := hlog.FromRequest(r)
	switch sourceType {
	case domain.PlexWebhook:
		return handlePlexWebhook(w, r)

	case domain.TautulliWebhook:
		return handleTautulli(w, r)

	default:
		log.Error().Str("sourceType", string(sourceType)).Msg("sourceType not supported")
		return nil, errors.New("unsupported source type")
	}
}

func handlePlexWebhook(w http.ResponseWriter, r *http.Request) (*domain.Plex, error) {
	log := hlog.FromRequest(r)
	if err := r.ParseMultipartForm(0); err != nil {
		http.Error(w, "received bad request", http.StatusBadRequest)
		log.Debug().Err(err).Msg("received bad request")
		return nil, err
	}

	ps := r.PostFormValue("payload")
	if ps == "" {
		log.Info().Msg("Received empty payload from Plex, webhook added successfully.")
		w.WriteHeader(http.StatusNoContent)
		return nil, errors.New("empty paylod")
	}

	log.Debug().RawJSON("rawPlexPayload", []byte(ps)).Msg("")
	return domain.NewPlexWebhook([]byte(ps))
}

func handleTautulli(w http.ResponseWriter, r *http.Request) (*domain.Plex, error) {
	log := hlog.FromRequest(r)
	ps, err := readRequest(r)
	if err != nil {
		http.Error(w, InternalServerError, http.StatusInternalServerError)
		log.Debug().Err(err).Msg(InternalServerError)
		return nil, err
	}

	log.Debug().RawJSON("rawPlexPayload", []byte(ps)).Msg("")
	return domain.ToPlex([]byte(ps))
}

func normalizeBaseUrl(baseUrl string) (string, string) {
	// Special case: If baseUrl is exactly "/", return "/" for both
	if baseUrl == "/" {
		return "/", "/"
	}

	// Ensure baseUrl starts and ends with "/"
	normalizedBaseUrl := "/" + strings.Trim(baseUrl, "/") + "/"

	// Remove trailing "/" for webBase unless it's the root path "/"
	webBase := strings.TrimSuffix(normalizedBaseUrl, "/")

	return normalizedBaseUrl, webBase
}

func generateClientId() string {
	const prefix = "shinkro-"
	const byteLength = 8

	randomBytes := make([]byte, byteLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return fmt.Sprintf("%s%x", prefix, randomBytes)
	}

	return fmt.Sprintf("%s%s", prefix, hex.EncodeToString(randomBytes))
}

func generateRandomIV() ([]byte, error) {
	iv := make([]byte, 12) // 12 bytes for AES-GCM standard
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	return iv, nil
}

func generatePKCE(length int) (verifier, challenge string, err error) {
	if length < 43 || length > 128 {
		return "", "", errors.New("length not supported")
	}

	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", errors.Errorf("failed to generate random bytes: %v", err)
	}

	verifier = base64.URLEncoding.EncodeToString(randomBytes)
	verifier = verifier[:length]

	//Waiting for support from MAL side
	// s256 := sha256.New()
	// s256.Write([]byte(verifier))
	// challenge = base64.URLEncoding.EncodeToString(s256.Sum(nil))
	// challenge = base64.RawURLEncoding.EncodeToString(s256.Sum(nil))

	challenge = verifier
	return verifier, challenge, nil
}

func generateState(l int) (string, error) {
	random := make([]byte, l)
	_, err := rand.Read(random)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate random string")
	}

	return base64.URLEncoding.EncodeToString(random)[:l], nil
}
