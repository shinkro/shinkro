package syncinspector

import (
	"context"
	"database/sql"
	"time"
)

// SourceSchema retains the identity and first local observation of immutable
// source generations. Selected rules are embedded in their attempt traces.
const SourceSchema = `
CREATE TABLE IF NOT EXISTS sync_source_snapshot (
 kind TEXT NOT NULL, content_hash TEXT NOT NULL, observed_at TEXT NOT NULL,
 PRIMARY KEY(kind,content_hash)
);
CREATE TABLE IF NOT EXISTS sync_source_check (
 kind TEXT PRIMARY KEY, refreshed_at TEXT NOT NULL
);
`

type SourceSnapshot struct {
	Kind          string     `json:"kind"`
	Label         string     `json:"label"`
	ContentHash   string     `json:"contentHash"`
	Entries       int        `json:"entries"`
	ObservedAt    time.Time  `json:"observedAt"`
	LastRefreshAt *time.Time `json:"lastRefreshAt,omitempty"`
}

func (s *Service) recordSource(ctx context.Context, kind, label string, entries int, content interface{}) (SourceSnapshot, error) {
	snapshot := SourceSnapshot{Kind: kind, Label: label, ContentHash: hash(content), Entries: entries}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sync_source_snapshot(kind,content_hash,observed_at) VALUES(?,?,?) ON CONFLICT(kind,content_hash) DO NOTHING`, kind, snapshot.ContentHash, s.now().Format(time.RFC3339Nano)); err != nil {
		return snapshot, err
	}
	var observed string
	if err := s.db.QueryRowContext(ctx, `SELECT observed_at FROM sync_source_snapshot WHERE kind=? AND content_hash=?`, kind, snapshot.ContentHash).Scan(&observed); err != nil {
		return snapshot, err
	}
	var err error
	snapshot.ObservedAt, err = time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return snapshot, err
	}
	var refreshed string
	err = s.db.QueryRowContext(ctx, `SELECT refreshed_at FROM sync_source_check WHERE kind=?`, kind).Scan(&refreshed)
	if err == nil {
		value, parseErr := time.Parse(time.RFC3339Nano, refreshed)
		if parseErr != nil {
			return snapshot, parseErr
		}
		snapshot.LastRefreshAt = &value
	} else if err != sql.ErrNoRows {
		return snapshot, err
	}
	return snapshot, nil
}

func (s *Service) catalogueSource(ctx context.Context) (SourceSnapshot, error) {
	// Hash the fields that actually drive resolution, in a stable order. A fresh
	// read also notices scheduled catalogue imports outside the inspector.
	rows, err := s.db.QueryContext(ctx, `SELECT mal_id,COALESCE(title,''),COALESCE(en_title,''),COALESCE(anidb_id,0),COALESCE(tvdb_id,0),COALESCE(tmdb_id,0),COALESCE(type,'') FROM anime ORDER BY mal_id`)
	if err != nil {
		return SourceSnapshot{}, err
	}
	type row struct {
		MALID             int
		Title, English    string
		AniDB, TVDB, TMDB int
		Type              string
	}
	content := []row{}
	for rows.Next() {
		var v row
		if err = rows.Scan(&v.MALID, &v.Title, &v.English, &v.AniDB, &v.TVDB, &v.TMDB, &v.Type); err != nil {
			rows.Close()
			return SourceSnapshot{}, err
		}
		content = append(content, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return SourceSnapshot{}, err
	}
	return s.recordSource(ctx, "catalogue", "ShinkroDB catalogue", len(content), content)
}

// Refresh timestamps mean a successful explicit upstream refresh, not merely
// reading an existing cache. Unknown upstream freshness is left unknown.
func (s *Service) markSourcesRefreshed(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := s.now().Format(time.RFC3339Nano)
	for _, kind := range []string{"catalogue", "tvdb", "tmdb"} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO sync_source_check(kind,refreshed_at) VALUES(?,?) ON CONFLICT(kind) DO UPDATE SET refreshed_at=excluded.refreshed_at`, kind, stamp); err != nil {
			return err
		}
	}
	return tx.Commit()
}
