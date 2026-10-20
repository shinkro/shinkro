package syncinspector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/stretchr/testify/require"
)

func TestMALAdapterBindsAccountBeforeWriteAndClassifiesResponse(t *testing.T) {
	var patchCount atomic.Int32
	var detailsStatus atomic.Int32
	detailsStatus.Store(http.StatusOK)
	var responseStatus atomic.Int32
	responseStatus.Store(http.StatusOK)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/users/@me" {
			_, _ = w.Write([]byte(`{"id":42,"name":"fixture"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/anime/384" {
			if detailsStatus.Load() != http.StatusOK {
				w.WriteHeader(int(detailsStatus.Load()))
				_, _ = w.Write([]byte(`{"error":"read unavailable"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":384,"title":"Actual title","num_episodes":24,"my_list_status":{"status":"watching","num_episodes_watched":7,"score":9,"is_rewatching":false,"num_times_rewatched":2,"start_date":"2026-01-02","finish_date":""}}`))
			return
		}
		if r.Method == "PATCH" {
			patchCount.Add(1)
			status := int(responseStatus.Load())
			w.WriteHeader(status)
			if status == 200 {
				_, _ = w.Write([]byte(`{"status":"watching","num_episodes_watched":7,"score":8}`))
			} else {
				_, _ = w.Write([]byte(`{"error":"fixture rejection"}`))
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	base, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	adapter := MALAdapter{Client: func(context.Context) (*mal.Client, error) {
		client := mal.NewClient(server.Client())
		client.BaseURL = base
		return client, nil
	}}
	ctx := context.Background()
	_, err = adapter.Write(ctx, "99", 384, []Change{{Field: "episodesWatched", Before: 0, After: 7}})
	require.Equal(t, "ACCOUNT_CHANGED", err.(*Error).Code)
	require.Zero(t, patchCount.Load())
	state, err := adapter.Write(ctx, "42", 384, []Change{{Field: "episodesWatched", Before: 0, After: 7}})
	require.NoError(t, err)
	require.Equal(t, 7, state.EpisodesWatched)
	require.Equal(t, 9, state.Score)
	require.Equal(t, 2, state.RewatchCount)
	require.Equal(t, "confirmation_read", state.EvidenceSource)
	require.EqualValues(t, 1, patchCount.Load())
	for _, code := range []int{400, 401, 403, 404, 429} {
		responseStatus.Store(int32(code))
		_, err = adapter.Write(ctx, "42", 384, []Change{{Field: "episodesWatched", Before: 0, After: 7}})
		require.Error(t, err)
		_, typed := err.(*Error)
		require.True(t, typed, "HTTP %d must be a definite rejection", code)
	}
	responseStatus.Store(500)
	_, err = adapter.Write(ctx, "42", 384, []Change{{Field: "episodesWatched", Before: 0, After: 7}})
	require.Error(t, err)
	_, typed := err.(*Error)
	require.False(t, typed, "HTTP 500 keeps uncertain outcome for coordinator reconciliation")
	responseStatus.Store(200)
	detailsStatus.Store(503)
	missing, err := adapter.Write(ctx, "42", 384, []Change{{Field: "episodesWatched", Before: 0, After: 7}})
	require.Error(t, err)
	require.Equal(t, "UNCERTAIN_WRITE", err.(*Error).Code)
	require.Equal(t, MALState{}, missing)

}
