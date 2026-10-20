package syncinspector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSourceGenerationsPreserveHistoryWithoutInvalidatingUnrelatedChanges(t *testing.T) {
	s, db, _, _, ctx := fixture(t, true)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	_, err := db.Exec(`INSERT INTO anime(mal_id,anidb_id,title,type) VALUES(63367,19859,'Beerus','tv')`)
	require.NoError(t, err)
	seed(t, s, Input{EventID: 1, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2})
	first, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, first.Ready, first.Message)
	require.Len(t, first.Resolution.Sources, 1)
	observed := first.Resolution.Sources[0]
	require.Equal(t, now, observed.ObservedAt)
	require.Nil(t, observed.LastRefreshAt, "a cache observation is not an upstream refresh")
	require.NotEmpty(t, observed.ContentHash)

	now = now.Add(time.Minute)
	second, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.Equal(t, observed.ObservedAt, second.Resolution.Sources[0].ObservedAt)
	require.True(t, samePreview(first, second))

	_, err = db.Exec(`INSERT INTO anime(mal_id,anidb_id,title,type) VALUES(384,2400,'Gantz','tv')`)
	require.NoError(t, err)
	third, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.NotEqual(t, observed.ContentHash, third.Resolution.Sources[0].ContentHash)
	require.True(t, samePreview(first, third), "an unrelated catalogue addition must not invalidate the approved target and patch")
	require.NoError(t, s.Refresh(ctx))
	fourth, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.NotNil(t, fourth.Resolution.Sources[0].LastRefreshAt)
	require.Equal(t, now, *fourth.Resolution.Sources[0].LastRefreshAt)
	var saved Preview
	require.NoError(t, readJSON(ctx, db, `SELECT payload FROM sync_preview WHERE id=?`, &saved, first.ID))
	require.Equal(t, observed, saved.Resolution.Sources[0], "saved historical provenance is immutable")
}

func TestFailedRefreshDoesNotClaimNewSourceFreshness(t *testing.T) {
	s, db, _, _, ctx := fixture(t, false)
	require.NoError(t, s.Refresh(ctx))
	var initial string
	require.NoError(t, db.QueryRow(`SELECT refreshed_at FROM sync_source_check WHERE kind='catalogue'`).Scan(&initial))
	s.refresh = func(_ context.Context) error { return errors.New("provider unavailable") }
	require.Error(t, s.Refresh(ctx))
	var after string
	require.NoError(t, db.QueryRow(`SELECT refreshed_at FROM sync_source_check WHERE kind='catalogue'`).Scan(&after))
	require.Equal(t, initial, after)
}
