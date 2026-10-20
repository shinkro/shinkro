package syncinspector

import (
	"context"
	"reflect"
	"time"
)

// plan only produces absolute, monotonic field intents. A replay never invents
// a new watch cycle, overwrites an old rating, or changes unrelated fields.
func plan(in Input, r Resolution, before MALState, now time.Time) (MALState, []Change, error) {
	after := before
	changes := []Change{}
	eventDate := in.ReceivedAt
	if eventDate.IsZero() {
		eventDate = now
	}
	add := func(field string, b, a interface{}) {
		if !reflect.DeepEqual(b, a) {
			changes = append(changes, Change{field, b, a})
		}
	}
	if in.Event == "media.rate" {
		if !before.UpdatedAt.IsZero() && before.UpdatedAt.After(in.ReceivedAt) {
			return after, changes, fail("STALE_RATING", "MAL changed after this rating event. Rate the title again in Plex to avoid overwriting newer activity.")
		}
		score := int(in.Rating)
		if score < 0 || score > 10 {
			return after, changes, fail("INVALID_RATING", "The rating must be between 0 and 10.")
		}
		after.Score = score
		add("score", before.Score, after.Score)
		return after, changes, nil
	}
	if in.Event != "media.scrobble" {
		return after, changes, fail("INVALID_INPUT", "Unsupported event type.")
	}
	if r.Episode < 1 || before.TotalEpisodes > 0 && r.Episode > before.TotalEpisodes {
		return after, changes, fail("INVALID_EPISODE", "The mapped episode is outside the MAL title's episode count.")
	}
	if r.Episode <= before.EpisodesWatched {
		return after, changes, nil
	}
	if before.IsRewatching && !before.UpdatedAt.IsZero() && before.UpdatedAt.After(in.ReceivedAt) {
		return after, changes, fail("STALE_WATCH_CYCLE", "This event predates activity in the current rewatch cycle. Play the intended episode again in Plex to confirm current-cycle progress.")
	}
	// Existing rewatch cycles can progress monotonically; a completion never adds
	// a rewatch count automatically because duplicated source events lack cycle IDs.
	after.EpisodesWatched = r.Episode
	add("episodesWatched", before.EpisodesWatched, after.EpisodesWatched)
	if before.Status != "completed" {
		if before.TotalEpisodes > 0 && r.Episode == before.TotalEpisodes {
			after.Status = "completed"
			if after.FinishDate == "" {
				after.FinishDate = eventDate.Format("2006-01-02")
				add("finishDate", before.FinishDate, after.FinishDate)
			}
		} else {
			after.Status = "watching"
		}
		add("status", before.Status, after.Status)
	}
	if before.EpisodesWatched == 0 && r.Episode == 1 && after.StartDate == "" {
		after.StartDate = eventDate.Format("2006-01-02")
		add("startDate", before.StartDate, after.StartDate)
	}
	return after, changes, nil
}
func relevantState(s MALState) string {
	return hash(struct {
		Status          string
		Episodes, Score int
		Rewatch         bool
		Count           int
		Start, Finish   string
		Updated         time.Time
	}{s.Status, s.EpisodesWatched, s.Score, s.IsRewatching, s.RewatchCount, s.StartDate, s.FinishDate, s.UpdatedAt})
}
func (s *Service) evaluate(ctx context.Context, in Input, c *Correction) Preview {
	now := s.now()
	p := Preview{ActorID: actor(ctx), ID: id(), EventID: in.EventID, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute), WritesEnabled: s.writes, Changes: []Change{}, Warnings: []string{}, Correction: c, Resolution: Resolution{Steps: []Step{}}}
	block := func(err error) Preview {
		p.Code = "INTERNAL_ERROR"
		p.Message = "The sync could not be evaluated."
		if e, ok := err.(*Error); ok {
			p.Code = e.Code
			p.Message = e.Message
		}
		return p
	}
	if ok, reason := eligible(in); !ok {
		return block(fail("LEGACY_INPUT", reason))
	}
	warnings, err := s.currentEligibility(ctx, in)
	if err != nil {
		return block(err)
	}
	p.Warnings = append(p.Warnings, warnings...)
	r, err := s.resolve(ctx, in, c)
	p.Resolution = r
	if err != nil {
		return block(err)
	}
	account, err := s.mal.Account(ctx)
	if err != nil {
		return block(fail("MAL_AUTH_FAILED", "Reconnect your MyAnimeList account or retry when MAL is available."))
	}
	p.AccountID = account
	before, err := s.mal.Read(ctx, r.MALID)
	if err != nil {
		return block(fail("MAL_API_FETCH_FAILED", "Current MAL state could not be read. No changes were made."))
	}
	p.Before = &before
	p.Resolution.Title = before.Title
	after, changes, err := plan(in, r, before, now)
	p.After = &after
	p.Changes = changes
	if err != nil {
		return block(err)
	}
	if before.Status == "completed" {
		p.Warnings = append(p.Warnings, "This action will not start a rewatch or increment the rewatch count.")
	}
	if c != nil {
		p.Warnings = append(p.Warnings, "If this event previously updated a different MAL title, that entry is left unchanged.")
	}
	if !s.writes {
		p.Warnings = append(p.Warnings, "MAL writes are disabled on this instance. You can inspect, preview, and save local rules.")
	}
	p.Ready = true
	if len(changes) == 0 {
		p.Message = "No change needed. MAL already has this progress or rating."
	} else {
		p.Message = "Review the proposed MAL changes before applying."
	}
	return p
}
func (s *Service) Preview(ctx context.Context, eventID int64, c *Correction) (Preview, error) {
	s.writer.Lock()
	defer s.writer.Unlock()
	in, _, err := s.input(ctx, eventID)
	if err != nil {
		return Preview{}, err
	}
	p := s.evaluate(ctx, in, c)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sync_preview(id,event_id,payload,created_at) VALUES(?,?,?,?)`, p.ID, eventID, marshal(p), p.CreatedAt)
	return p, err
}

// Live policy preserves legitimate fresh rewatch cycles while replay remains
// monotonic. An already completed duplicate cannot increment the count twice.
func (s *Service) evaluateLive(ctx context.Context, in Input) Preview {
	p := s.evaluate(ctx, in, nil)
	if !p.Ready || p.Before == nil || in.Event != "media.scrobble" || in.Version < 1 || s.now().Sub(in.ReceivedAt) > 10*time.Minute {
		return p
	}
	b := *p.Before
	after := *p.After
	if b.Status == "completed" && !b.IsRewatching && p.Resolution.Episode == 1 && b.TotalEpisodes != 1 {
		after.EpisodesWatched = 1
		after.IsRewatching = true
		p.Changes = []Change{{"episodesWatched", b.EpisodesWatched, 1}, {"isRewatching", false, true}}
	} else if b.IsRewatching && b.TotalEpisodes > 0 && p.Resolution.Episode == b.TotalEpisodes && b.EpisodesWatched < b.TotalEpisodes {
		after.IsRewatching = false
		after.RewatchCount = b.RewatchCount + 1
		p.Changes = append(p.Changes, Change{"isRewatching", true, false}, Change{"rewatchCount", b.RewatchCount, after.RewatchCount})
	}
	p.After = &after
	return p
}
