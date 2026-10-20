package syncinspector

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"
)

const Schema = `
CREATE TABLE IF NOT EXISTS sync_input (event_id INTEGER PRIMARY KEY REFERENCES plex_payload(id) ON DELETE CASCADE, input TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sync_preview (id TEXT PRIMARY KEY,event_id INTEGER NOT NULL REFERENCES plex_payload(id) ON DELETE CASCADE, payload TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sync_job (id TEXT PRIMARY KEY,event_id INTEGER NOT NULL REFERENCES plex_payload(id) ON DELETE CASCADE, preview_id TEXT UNIQUE, kind TEXT NOT NULL,status TEXT NOT NULL,payload TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS sync_job_event ON sync_job(event_id,created_at);
CREATE TABLE IF NOT EXISTS sync_attempt (id TEXT PRIMARY KEY,job_id TEXT NOT NULL,event_id INTEGER NOT NULL REFERENCES plex_payload(id) ON DELETE CASCADE,payload TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS sync_attempt_event ON sync_attempt(event_id,created_at);
CREATE TABLE IF NOT EXISTS sync_override_revision (id TEXT NOT NULL,revision INTEGER NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(id,revision));
CREATE TABLE IF NOT EXISTS sync_override (id TEXT PRIMARY KEY,payload TEXT NOT NULL,active INTEGER NOT NULL,created_at TEXT NOT NULL);
` + SourceSchema

func id() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func hash(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func marshal(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
func readJSON(ctx context.Context, db *sql.DB, query string, dst interface{}, args ...interface{}) error {
	var b string
	if err := db.QueryRowContext(ctx, query, args...).Scan(&b); err != nil {
		return err
	}
	return json.Unmarshal([]byte(b), dst)
}
func (s *Service) putJob(ctx context.Context, j Job) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sync_job SET status=?,payload=?,preview_id=NULLIF(?,'') WHERE id=?`, j.Status, marshal(j), j.PreviewID, j.ID)
	return err
}
func (s *Service) Job(ctx context.Context, key string) (Job, error) {
	var j Job
	err := readJSON(ctx, s.db, `SELECT payload FROM sync_job WHERE id=?`, &j, key)
	return j, err
}
func (s *Service) Overrides(ctx context.Context) ([]Override, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM sync_override WHERE active=1 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Override{}
	for rows.Next() {
		var raw string
		var v Override
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func sameRuleIdentity(a, b Override) bool {
	return a.Source == b.Source && a.SourceID == b.SourceID && a.Season == b.Season && a.Library == b.Library && a.Account == b.Account && a.MediaType == b.MediaType
}
func scope(o Override, in Input) bool {
	return o.Active && sameRuleIdentity(o, Override{Source: in.Source, SourceID: in.SourceID, Season: in.Season, Library: in.Library, Account: in.Account, MediaType: in.MediaType}) && (o.AnyServer || o.ServerID == in.ServerID)
}

// An all-server rule intersects every server-specific rule with the same
// account/library/provider scope. It never silently wins over a narrower rule.
func scopesIntersect(a, b Override) bool {
	return sameRuleIdentity(a, b) && (a.AnyServer || b.AnyServer || a.ServerID == b.ServerID)
}
func (s *Service) SaveOverride(ctx context.Context, req Override) (Override, error) {
	s.writer.Lock()
	defer s.writer.Unlock()
	req.ID = ""
	req.Revision = 0
	req.CreatedAt = time.Time{}
	req.Active = true
	impact, err := s.overrideImpact(ctx, req)
	if err != nil {
		return req, err
	}
	if impact.Scope.ServerID == "" && !impact.Scope.AnyServer {
		return req, fail("SCOPE_REQUIRED", "This event has no server identity. Explicitly choose all servers in this library and Plex account before saving a rule.")
	}
	if !impact.Ready {
		return req, fail("RULE_CONFLICT", "This range overlaps an active local rule.")
	}
	if req.ImpactFingerprint == "" || req.ImpactFingerprint != impact.Fingerprint {
		return req, fail("STALE_RULE_PREVIEW", "Preview and review this exact rule scope before saving.")
	}
	o := impact.Scope
	o.ID = id()
	o.Revision = 1
	o.CreatedAt = s.now()
	o.Active = true
	o.ActorID = actor(ctx)
	o.ImpactFingerprint = ""
	return o, s.persistOverride(ctx, o, true)
}
func (s *Service) persistOverride(ctx context.Context, o Override, create bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if create {
		_, err = tx.ExecContext(ctx, `INSERT INTO sync_override(id,payload,active,created_at) VALUES(?,?,?,?)`, o.ID, marshal(o), o.Active, o.CreatedAt)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE sync_override SET active=?,payload=? WHERE id=?`, o.Active, marshal(o), o.ID)
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sync_override_revision(id,revision,payload) VALUES(?,?,?)`, o.ID, o.Revision, marshal(o)); err != nil {
		return err
	}
	return tx.Commit()
}
