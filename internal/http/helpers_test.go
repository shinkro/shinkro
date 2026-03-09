package http

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseUrl(t *testing.T) {
	tests := []struct {
		name           string
		baseUrl        string
		expectedNormal string
		expectedWeb    string
	}{
		{
			name:           "root path",
			baseUrl:        "/",
			expectedNormal: "/",
			expectedWeb:    "/",
		},
		{
			name:           "simple path",
			baseUrl:        "shinkro",
			expectedNormal: "/shinkro/",
			expectedWeb:    "/shinkro",
		},
		{
			name:           "path with leading slash",
			baseUrl:        "/shinkro",
			expectedNormal: "/shinkro/",
			expectedWeb:    "/shinkro",
		},
		{
			name:           "path with trailing slash",
			baseUrl:        "shinkro/",
			expectedNormal: "/shinkro/",
			expectedWeb:    "/shinkro",
		},
		{
			name:           "path with both slashes",
			baseUrl:        "/shinkro/",
			expectedNormal: "/shinkro/",
			expectedWeb:    "/shinkro",
		},
		{
			name:           "nested path",
			baseUrl:        "/app/shinkro",
			expectedNormal: "/app/shinkro/",
			expectedWeb:    "/app/shinkro",
		},
		{
			name:           "empty string",
			baseUrl:        "",
			expectedNormal: "//",
			expectedWeb:    "/",
		},
		{
			name:           "multiple slashes",
			baseUrl:        "///shinkro///",
			expectedNormal: "/shinkro/",
			expectedWeb:    "/shinkro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normal, web := normalizeBaseUrl(tt.baseUrl)
			assert.Equal(t, tt.expectedNormal, normal)
			assert.Equal(t, tt.expectedWeb, web)
		})
	}
}

func TestGenerateClientId(t *testing.T) {
	// Test that it generates a client ID with correct prefix
	clientId := generateClientId()
	assert.True(t, strings.HasPrefix(clientId, "shinkro-"))
	assert.Greater(t, len(clientId), len("shinkro-"))

	// Test that it generates unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateClientId()
		assert.False(t, ids[id], "should generate unique IDs")
		ids[id] = true
	}
}

func TestGenerateRandomIV(t *testing.T) {
	// Test that it generates a 12-byte IV
	iv, err := generateRandomIV()
	require.NoError(t, err)
	assert.Equal(t, 12, len(iv))

	// Test that it generates unique IVs
	ivs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		iv, err := generateRandomIV()
		require.NoError(t, err)
		ivStr := string(iv)
		assert.False(t, ivs[ivStr], "should generate unique IVs")
		ivs[ivStr] = true
	}
}

func TestGeneratePKCE(t *testing.T) {
	tests := []struct {
		name          string
		length        int
		expectedError bool
		validate      func(*testing.T, string, string)
	}{
		{
			name:          "valid minimum length",
			length:        43,
			expectedError: false,
			validate: func(t *testing.T, verifier, challenge string) {
				assert.Equal(t, 43, len(verifier))
				assert.Equal(t, verifier, challenge) // Currently challenge equals verifier
			},
		},
		{
			name:          "valid maximum length",
			length:        128,
			expectedError: false,
			validate: func(t *testing.T, verifier, challenge string) {
				assert.Equal(t, 128, len(verifier))
				assert.Equal(t, verifier, challenge)
			},
		},
		{
			name:          "valid middle length",
			length:        64,
			expectedError: false,
			validate: func(t *testing.T, verifier, challenge string) {
				assert.Equal(t, 64, len(verifier))
				assert.Equal(t, verifier, challenge)
			},
		},
		{
			name:          "too short",
			length:        42,
			expectedError: true,
		},
		{
			name:          "too long",
			length:        129,
			expectedError: true,
		},
		{
			name:          "zero length",
			length:        0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, challenge, err := generatePKCE(tt.length)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, verifier)
				assert.Empty(t, challenge)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, verifier)
				assert.NotEmpty(t, challenge)
				if tt.validate != nil {
					tt.validate(t, verifier, challenge)
				}
			}
		})
	}

	// Test uniqueness
	verifiers := make(map[string]bool)
	for i := 0; i < 50; i++ {
		verifier, _, err := generatePKCE(64)
		require.NoError(t, err)
		assert.False(t, verifiers[verifier], "should generate unique verifiers")
		verifiers[verifier] = true
	}
}

func TestGenerateState(t *testing.T) {
	tests := []struct {
		name          string
		length        int
		expectedError bool
		validate      func(*testing.T, string)
	}{
		{
			name:          "valid length",
			length:        32,
			expectedError: false,
			validate: func(t *testing.T, state string) {
				assert.Equal(t, 32, len(state))
			},
		},
		{
			name:          "short length",
			length:        16,
			expectedError: false,
			validate: func(t *testing.T, state string) {
				assert.Equal(t, 16, len(state))
			},
		},
		{
			name:          "long length",
			length:        64,
			expectedError: false,
			validate: func(t *testing.T, state string) {
				assert.Equal(t, 64, len(state))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := generateState(tt.length)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, state)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, state)
				if tt.validate != nil {
					tt.validate(t, state)
				}
			}
		})
	}

	// Test uniqueness
	states := make(map[string]bool)
	for i := 0; i < 50; i++ {
		state, err := generateState(32)
		require.NoError(t, err)
		assert.False(t, states[state], "should generate unique states")
		states[state] = true
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		expectedSource domain.PlexPayloadSource
	}{
		{
			name:           "multipart form data",
			contentType:    "multipart/form-data; boundary=----WebKitFormBoundary",
			expectedSource: domain.PlexWebhook,
		},
		{
			name:           "application json",
			contentType:    "application/json",
			expectedSource: domain.TautulliWebhook,
		},
		{
			name:           "application json with charset",
			contentType:    "application/json; charset=utf-8",
			expectedSource: domain.TautulliWebhook,
		},
		{
			name:           "empty content type",
			contentType:    "",
			expectedSource: "",
		},
		{
			name:           "unknown content type",
			contentType:    "text/plain",
			expectedSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.Header.Set("Content-Type", tt.contentType)
			result := contentType(req)
			assert.Equal(t, tt.expectedSource, result)
		})
	}
}

func TestReadRequest(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		expectedError bool
	}{
		{
			name:          "valid body",
			body:          `{"test": "data"}`,
			expectedError: false,
		},
		{
			name:          "empty body",
			body:          "",
			expectedError: false,
		},
		{
			name:          "large body",
			body:          strings.Repeat("a", 10000),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
			result, err := readRequest(req)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.body, result)
			}
		})
	}
}

func TestHandlePlexWebhook(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedError bool
		validate      func(*testing.T, *domain.Plex)
	}{
		{
			name:          "valid plex webhook",
			payload:       testdata.RawPlexWebhookHAMAEpisode(),
			expectedError: false,
			validate: func(t *testing.T, p *domain.Plex) {
				assert.NotNil(t, p)
				assert.Equal(t, domain.PlexScrobbleEvent, p.Event)
				assert.Equal(t, "One Piece", p.Metadata.GrandparentTitle)
			},
		},
		{
			name:          "empty payload",
			payload:       "",
			expectedError: true,
		},
		{
			name:          "invalid json",
			payload:       "{invalid json}",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipartWriter(t, body, tt.payload)
			writer.Close()

			req := httptest.NewRequest("POST", "/", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			result, err := handlePlexWebhook(w, req)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestHandleTautulli(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedError bool
		validate      func(*testing.T, *domain.Plex)
	}{
		{
			name:          "valid tautulli payload",
			payload:       testdata.RawTautulliEpisode(),
			expectedError: false,
			validate: func(t *testing.T, p *domain.Plex) {
				assert.NotNil(t, p)
				assert.Equal(t, domain.TautulliWebhook, p.Source)
				assert.Equal(t, domain.PlexScrobbleEvent, p.Event)
			},
		},
		{
			name:          "invalid json",
			payload:       "{invalid}",
			expectedError: true,
		},
		{
			name:          "empty payload",
			payload:       "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			result, err := handleTautulli(w, req)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestParsePayloadBySourceType(t *testing.T) {
	tests := []struct {
		name          string
		sourceType    domain.PlexPayloadSource
		setupRequest  func() *http.Request
		expectedError bool
	}{
		{
			name:       "plex webhook",
			sourceType: domain.PlexWebhook,
			setupRequest: func() *http.Request {
				body := &bytes.Buffer{}
				writer := multipartWriter(t, body, testdata.RawPlexWebhookHAMAEpisode())
				writer.Close()
				req := httptest.NewRequest("POST", "/", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req
			},
			expectedError: false,
		},
		{
			name:       "tautulli webhook",
			sourceType: domain.TautulliWebhook,
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/", bytes.NewBufferString(testdata.RawTautulliEpisode()))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedError: false,
		},
		{
			name:          "unsupported source type",
			sourceType:    domain.PlexPayloadSource("unknown"),
			setupRequest:  func() *http.Request { return httptest.NewRequest("POST", "/", nil) },
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			w := httptest.NewRecorder()

			result, err := parsePayloadBySourceType(w, req, tt.sourceType)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// Helper function to create multipart form data
func multipartWriter(t *testing.T, body *bytes.Buffer, payload string) *multipart.Writer {
	writer := multipart.NewWriter(body)
	err := writer.WriteField("payload", payload)
	require.NoError(t, err)
	return writer
}
