package syncinspector

import (
	"context"
	"encoding/json"
	"time"
)

type Impact struct {
	Scope            Override   `json:"scope"`
	AffectedEventIDs []int64    `json:"affectedEventIds"`
	Conflicts        []Override `json:"conflicts"`
	Ready            bool       `json:"ready"`
	Warnings         []string   `json:"warnings"`
	Fingerprint      string     `json:"fingerprint"`
}

func (s *Service) OverrideImpact(ctx context.Context, o Override) (Impact, error) {
	s.writer.Lock()
	defer s.writer.Unlock()
	return s.overrideImpact(ctx, o)
}
func (s *Service) overrideImpact(ctx context.Context, o Override) (Impact, error) {
	d, err := s.Detail(ctx, o.EventID)
	if err != nil {
		return Impact{}, err
	}
	in := d.Input
	if !d.ReplayEligible {
		return Impact{}, fail("LEGACY_INPUT", d.ReplayReason)
	}
	if err = validOverride(o, in); err != nil {
		return Impact{}, err
	}
	o.Source = in.Source
	o.SourceID = in.SourceID
	o.Season = in.Season
	o.Library = in.Library
	o.Account = in.Account
	o.ServerID = in.ServerID
	o.MediaType = in.MediaType
	o.Active = true
	o.ActorID = actor(ctx)
	if o.AnyServer {
		o.ServerID = ""
	}
	result := Impact{Scope: o, AffectedEventIDs: []int64{}, Conflicts: []Override{}, Warnings: []string{}}
	if in.ServerID == "" && !o.AnyServer {
		result.Warnings = append(result.Warnings, "This historical event has no server identity. To save a rule for future events, explicitly select all servers in this library and Plex account. You can also use an event-only correction.")
		return result, nil
	}
	if o.AnyServer {
		result.Warnings = append(result.Warnings, "This rule applies to all servers within the displayed Plex account and library. Other accounts, libraries, providers, source titles and seasons are excluded.")
	}

	target, err := s.mal.Read(ctx, o.MALID)
	if err != nil {
		return Impact{}, fail("INVALID_MAL_TARGET", "The MAL title could not be verified. Check its ID and your MAL connection.")
	}
	if target.TotalEpisodes > 0 && o.MALEpisodeStart+o.SourceEpisodeEnd-o.SourceEpisodeStart > target.TotalEpisodes {
		return Impact{}, fail("INVALID_EPISODE", "This range exceeds the MAL title's known episode count.")
	}
	existing, err := s.Overrides(ctx)
	if err != nil {
		return result, err
	}
	for _, v := range existing {
		if v.ID != o.ID && scopesIntersect(v, o) && v.SourceEpisodeStart <= o.SourceEpisodeEnd && o.SourceEpisodeStart <= v.SourceEpisodeEnd {
			result.Conflicts = append(result.Conflicts, v)
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM plex_payload ORDER BY id`)
	if err != nil {
		return result, err
	}
	keys := []int64{}
	for rows.Next() {
		var key int64
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return result, err
		}
		keys = append(keys, key)
	}
	rows.Close()
	for _, key := range keys {
		v, _, err := s.input(ctx, key)
		if err != nil {
			return result, err
		}
		if scope(o, v) && v.Episode >= o.SourceEpisodeStart && v.Episode <= o.SourceEpisodeEnd {
			result.AffectedEventIDs = append(result.AffectedEventIDs, key)
		}
	}
	result.Ready = len(result.Conflicts) == 0
	clean := o
	clean.ImpactFingerprint = ""
	clean.CreatedAt = time.Time{}
	result.Fingerprint = hash(struct {
		Scope     Override
		Conflicts []Override
		Total     int
		Affected  []int64
	}{clean, result.Conflicts, target.TotalEpisodes, result.AffectedEventIDs})
	return result, nil
}
func validOverride(o Override, in Input) error {
	if o.MALID <= 0 || o.SourceEpisodeStart < 1 || o.SourceEpisodeEnd < o.SourceEpisodeStart || o.SourceEpisodeEnd > 100000 || o.MALEpisodeStart < 1 || o.MALEpisodeStart > 100000 || o.MALEpisodeStart+o.SourceEpisodeEnd-o.SourceEpisodeStart > 100000 {
		return fail("INVALID_RULE", "Enter a valid MAL ID and episode range.")
	}
	if in.Source == "myanimelist" {
		return fail("INVALID_RULE", "Direct MAL events support event-only corrections.")
	}
	if in.Source == "anidb" && in.Season != 1 {
		return fail("INVALID_RULE", "AniDB rules must explicitly target season one; later seasons use TVDB mappings.")
	}
	if in.MediaType == "movie" && (o.SourceEpisodeStart != 1 || o.SourceEpisodeEnd != 1 || o.MALEpisodeStart != 1) {
		return fail("INVALID_RULE", "Movie rules use episode one.")
	}
	return nil
}
func (s *Service) DeleteOverrideRevision(ctx context.Context, key string, revision int) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	var o Override
	if err := readJSON(ctx, s.db, `SELECT payload FROM sync_override WHERE id=?`, &o, key); err != nil {
		return err
	}
	if revision < 1 || o.Revision != revision {
		return fail("REVISION_CONFLICT", "This rule changed. Reload the rule list before editing it.")
	}
	o.Active = false
	o.Revision++
	o.ActorID = actor(ctx)
	return s.persistOverride(ctx, o, false)
}
func (s *Service) UpdateOverride(ctx context.Context, req Override) (Override, error) {
	s.writer.Lock()
	defer s.writer.Unlock()
	var old Override
	if err := readJSON(ctx, s.db, `SELECT payload FROM sync_override WHERE id=?`, &old, req.ID); err != nil {
		return req, err
	}
	if req.Revision != old.Revision {
		return old, fail("REVISION_CONFLICT", "This rule changed. Reload the rule list before editing it.")
	}
	if !req.Active {
		old.Active = false
		old.Revision++
		old.ActorID = actor(ctx)
		return old, s.persistOverride(ctx, old, false)
	}
	req.EventID = old.EventID
	impact, err := s.overrideImpact(ctx, req)
	if err != nil {
		return old, err
	}
	if impact.Scope.ServerID == "" && !impact.Scope.AnyServer {
		return old, fail("SCOPE_REQUIRED", "This event has no server identity. Explicitly choose all servers in this library and Plex account before saving a rule.")
	}
	if len(impact.Conflicts) > 0 && req.Active {
		return old, fail("RULE_CONFLICT", "The edited range overlaps another active rule.")
	}
	if req.Active && (req.ImpactFingerprint == "" || req.ImpactFingerprint != impact.Fingerprint) {
		return old, fail("STALE_RULE_PREVIEW", "Preview the edited rule scope before saving.")
	}
	updated := impact.Scope
	updated.ID = old.ID
	updated.Revision = old.Revision + 1
	updated.CreatedAt = old.CreatedAt
	updated.Active = req.Active
	updated.ActorID = actor(ctx)
	updated.ImpactFingerprint = ""
	return updated, s.persistOverride(ctx, updated, false)
}

// AllOverrides includes inactive revisions so a user can restore a disabled rule.
func (s *Service) AllOverrides(ctx context.Context) ([]Override, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM sync_override ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Override{}
	for rows.Next() {
		var raw string
		var o Override
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
