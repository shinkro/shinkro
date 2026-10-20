package database

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSyncInspectorUpgradePreservesHistoryAndDoesNotQueueLegacy(t *testing.T) {
	log := zerolog.Nop()
	db := NewDB(t.TempDir(), &log)
	defer db.Close()
	require.NoError(t, db.Migrate())
	_, err := db.handler.Exec(`DROP TABLE sync_override_revision;DROP TABLE sync_override;DROP TABLE sync_attempt;DROP TABLE sync_job;DROP TABLE sync_preview;DROP TABLE sync_input;PRAGMA user_version=2;INSERT INTO users(id,username,password)VALUES(1,'test-user','hash');INSERT INTO plex_payload(id,event,source,account_title,library_section_title,type)VALUES(1,'media.scrobble','Plex Webhook','viewer','Anime','episode')`)
	require.NoError(t, err)
	require.NoError(t, db.Migrate())
	var count int
	require.NoError(t, db.handler.QueryRow(`SELECT count(*) FROM users`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.handler.QueryRow(`SELECT count(*) FROM plex_payload`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.handler.QueryRow(`SELECT count(*) FROM sync_job`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Migrate())
}
func TestAnimeUpdateHistoryUsesDeterministicIDTieBreak(t *testing.T) {
	log := zerolog.Nop()
	db := NewDB(t.TempDir(), &log)
	defer db.Close()
	require.NoError(t, db.Migrate())
	repo := NewAnimeUpdateRepo(log, db)
	ctx := context.Background()
	stamp := time.Now().UTC()
	for _, ep := range []int{2, 3} {
		a := &domain.AnimeUpdate{PlexId: 1, MALId: 384, SourceDB: domain.MAL, SourceId: 384, EpisodeNum: ep, Timestamp: stamp, Status: domain.AnimeUpdateStatusSuccess}
		require.NoError(t, repo.Store(ctx, a))
	}
	a, err := repo.GetByPlexID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 3, a.EpisodeNum)
	a, err = repo.GetByID(ctx, &domain.GetAnimeUpdateRequest{Id: 1})
	require.NoError(t, err)
	require.Equal(t, 2, a.EpisodeNum)
	recent, err := repo.GetRecentUnique(ctx, 10)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	require.Equal(t, 3, recent[0].EpisodeNum)
}
