package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
)

func snapshotRepo(t *testing.T) (*DB, domain.AnimeRepo) {
	t.Helper()
	log := zerolog.Nop()
	db := NewDB(t.TempDir(), &log)
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db, NewAnimeRepo(log, db)
}

func TestReplaceAllRemovesStaleAnimeAndRestoresUnambiguousLookup(t *testing.T) {
	_, repo := snapshotRepo(t)
	ctx := context.Background()
	first := &domain.Anime{MALId: 384, MainTitle: "Gantz", AnimeType: "tv", AniDBId: 2400}
	second := &domain.Anime{MALId: 395, MainTitle: "Gantz 2nd Stage", AnimeType: "tv", AniDBId: 2400}
	if err := repo.StoreMultiple([]*domain.Anime{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceAll(ctx, []*domain.Anime{first}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, &domain.GetAnimeRequest{IDtype: domain.AniDB, Id: 2400})
	if err != nil || got.MALId != 384 {
		t.Fatalf("canonical AniDB lookup: %#v %v", got, err)
	}
	if _, err := repo.GetByID(ctx, &domain.GetAnimeRequest{IDtype: domain.MAL, Id: 395}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale row survived: %v", err)
	}
}

func TestReplaceAllRetainsPreviousSnapshotOnFailure(t *testing.T) {
	for _, kind := range []string{"empty", "nil entry", "invalid ID", "cancelled", "insert failure"} {
		t.Run(kind, func(t *testing.T) {
			db, repo := snapshotRepo(t)
			if err := repo.StoreMultiple([]*domain.Anime{{MALId: 1, MainTitle: "Existing"}}); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			rows := []*domain.Anime{{MALId: 2, MainTitle: "New"}}
			switch kind {
			case "empty":
				rows = nil
			case "nil entry":
				rows = []*domain.Anime{nil}
			case "invalid ID":
				rows[0].MALId = 0
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "insert failure":
				if _, err := db.handler.Exec(`CREATE TRIGGER reject_snapshot BEFORE INSERT ON anime WHEN NEW.mal_id=2 BEGIN SELECT RAISE(ABORT,'test insert failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if err := repo.ReplaceAll(ctx, rows); err == nil {
				t.Fatal("expected replacement failure")
			}
			got, err := repo.GetByID(context.Background(), &domain.GetAnimeRequest{IDtype: domain.MAL, Id: 1})
			if err != nil || got.MainTitle != "Existing" {
				t.Fatalf("previous snapshot changed: %#v %v", got, err)
			}
		})
	}
}
