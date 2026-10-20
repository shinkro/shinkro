package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerIdentityUsesConfiguredServerAndSanitizesFailures(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		body, identity string
	}{{"valid", 200, `{"MediaContainer":{"machineIdentifier":"fixture-server-id"}}`, "fixture-server-id"}, {"missing", 200, `{"MediaContainer":{}}`, ""}, {"invalid", 200, "private-response-must-not-leak", ""}, {"rejected", 403, "private-response-must-not-leak", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/identity", r.URL.Path)
				require.Equal(t, "application/json", r.Header.Get("Accept"))
				require.Equal(t, "fixture-token", r.Header.Get("X-Plex-Token"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewClient(Config{Url: server.URL, Token: "fixture-token", ClientID: "test-client"})
			identity, err := client.Identity(context.Background())
			if tc.identity != "" {
				require.NoError(t, err)
				require.Equal(t, tc.identity, identity)
			} else {
				require.Error(t, err)
				require.Empty(t, identity)
				require.NotContains(t, err.Error(), "private-response")
				require.NotContains(t, err.Error(), "fixture-token")
				require.NotContains(t, err.Error(), server.URL)
			}
		})
	}
}
