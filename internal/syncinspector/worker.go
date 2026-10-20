package syncinspector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/shinkro/shinkro/internal/domain"
)

func (s *Service) Replay(ctx context.Context, key int64, previewID string) (Job, error) {
	s.writer.Lock()
	defer s.writer.Unlock()
	if !s.writes {
		return Job{}, fail("WRITES_DISABLED", "MAL writes are disabled on this instance.")
	}
	in, _, err := s.input(ctx, key)
	if err != nil {
		return Job{}, err
	}
	if _, err = s.currentEligibility(ctx, in); err != nil {
		return Job{}, err
	}
	var prior Job
	if err := readJSON(ctx, s.db, `SELECT payload FROM sync_job WHERE preview_id=?`, &prior, previewID); err == nil {
		if prior.EventID != key {
			return Job{}, fail("INVALID_PREVIEW", "This preview belongs to a different event.")
		}
		return prior, nil
	} else if err != sql.ErrNoRows {
		return Job{}, err
	}

	var p Preview
	if err := readJSON(ctx, s.db, `SELECT payload FROM sync_preview WHERE id=? AND event_id=?`, &p, previewID, key); err != nil {
		return Job{}, fail("INVALID_PREVIEW", "Preview this event before applying changes.")
	}
	if p.ActorID != actor(ctx) {
		return Job{}, fail("INVALID_PREVIEW", "This preview belongs to another operator.")
	}
	if !p.Ready || !s.now().Before(p.ExpiresAt) {
		return Job{}, fail("STALE_PREVIEW", "This preview expired or is blocked. Create a fresh preview.")
	}
	var active Job
	if err := readJSON(ctx, s.db, `SELECT payload FROM sync_job WHERE event_id=? AND status IN ('queued','processing') ORDER BY created_at LIMIT 1`, &active, key); err == nil {
		var activePreview Preview
		if active.PreviewID != "" && readJSON(ctx, s.db, `SELECT payload FROM sync_preview WHERE id=?`, &activePreview, active.PreviewID) == nil && samePreview(p, activePreview) {
			return active, nil
		}
		return Job{}, fail("JOB_IN_PROGRESS", "This event already has an active sync with a different intent. Wait for its result, then preview the correction again.")
	} else if err != sql.ErrNoRows {
		return Job{}, err
	}
	if err := s.checkUncertain(ctx, key); err != nil {
		return Job{}, err
	}
	fresh := s.evaluate(ctx, in, p.Correction)
	if !samePreview(p, fresh) {
		return Job{}, fail("STALE_PREVIEW", "The account, mapping, or MAL state changed. Review a fresh preview.")
	}
	now := s.now()
	j := Job{ActorID: actor(ctx), ID: id(), EventID: key, Kind: "replay", Status: "queued", CreatedAt: now, UpdatedAt: now, PreviewID: p.ID, AccountID: p.AccountID}
	if p.Correction != nil {
		j.Kind = "correction"
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sync_job(id,event_id,preview_id,kind,status,payload,created_at) VALUES(?,?,?,?,?,?,?)`, j.ID, j.EventID, p.ID, j.Kind, j.Status, marshal(j), now)
	if err == nil {
		s.signal()
	}
	return j, err
}
func samePreview(p, q Preview) bool {
	return p.Ready && q.Ready && p.AccountID == q.AccountID && p.Resolution.Fingerprint == q.Resolution.Fingerprint && p.Before != nil && q.Before != nil && relevantState(*p.Before) == relevantState(*q.Before) && hash(p.Changes) == hash(q.Changes)
}
func (s *Service) run() {
	defer close(s.done)
	s.recover()
	s.signal()
	lastPrune := time.Time{}
	timer := time.NewTicker(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
		case <-timer.C:
		}
		if s.now().Sub(lastPrune) >= time.Hour {
			s.writer.Lock()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := s.prunePreviews(ctx, s.now())
			cancel()
			s.writer.Unlock()
			if err != nil {
				log.Printf("sync inspector preview pruning failed: %v", err)
			} else {
				lastPrune = s.now()
			}
		}
		s.recover()
		for {
			s.writer.Lock()
			var j Job
			err := readJSON(context.Background(), s.db, `SELECT payload FROM sync_job WHERE status='queued' ORDER BY created_at,id LIMIT 1`, &j)
			if err != nil {
				s.writer.Unlock()
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			executionErr := s.execute(ctx, j)
			cancel()
			s.writer.Unlock()
			if executionErr != nil {
				log.Printf("sync inspector persistence failure: %v", executionErr)
				break
			}
			select {
			case <-s.stop:
				return
			default:
			}
		}
	}
}
func (s *Service) recover() {
	s.writer.Lock()
	defer s.writer.Unlock()
	rows, err := s.db.Query(`SELECT payload FROM sync_job WHERE status='processing'`)
	if err != nil {
		return
	}
	jobs := []Job{}
	for rows.Next() {
		var raw string
		var j Job
		if rows.Scan(&raw) == nil && json.Unmarshal([]byte(raw), &j) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var p Preview
		if readJSON(ctx, s.db, `SELECT payload FROM sync_preview WHERE id=?`, &p, j.PreviewID) == nil {
			if s.reconcile(ctx, j, p) {
				cancel()
				continue
			}
		}
		if err := s.finish(ctx, j, nil, "needs_review", "UNCERTAIN_WRITE", "The app stopped during an update. Preview the event; this write must be reconciled before retrying."); err != nil {
			log.Printf("sync inspector recovery persistence failure: %v", err)
		}
		cancel()
	}
}
func (s *Service) checkUncertain(ctx context.Context, key int64) error {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM sync_job WHERE event_id=? AND status IN ('needs_review','processing')`, key)
	if err != nil {
		return err
	}
	jobs := []Job{}
	for rows.Next() {
		var raw string
		var j Job
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal([]byte(raw), &j); err != nil {
			rows.Close()
			return err
		}
		if j.Code == "UNCERTAIN_WRITE" || j.Status == "processing" {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		var p Preview
		if err := readJSON(ctx, s.db, `SELECT payload FROM sync_preview WHERE id=?`, &p, j.PreviewID); err != nil || !s.reconcile(ctx, j, p) {
			return fail("UNCERTAIN_WRITE", "A previous remote write has an uncertain outcome. MAL does not yet show its intended state; automatic resubmission is blocked.")
		}
	}
	return nil
}
func patchObserved(p Preview, remote MALState) bool {
	if p.After == nil {
		return false
	}
	for _, c := range p.Changes {
		switch c.Field {
		case "episodesWatched":
			if remote.EpisodesWatched < asInt(c.After) {
				return false
			}
		case "score":
			if remote.Score != asInt(c.After) {
				return false
			}
		case "status":
			if remote.Status != fmt.Sprint(c.After) {
				return false
			}
		case "isRewatching":
			if remote.IsRewatching != c.After.(bool) {
				return false
			}
		case "rewatchCount":
			if remote.RewatchCount < asInt(c.After) {
				return false
			}
		case "startDate":
			if remote.StartDate != c.After {
				return false
			}
		case "finishDate":
			if remote.FinishDate != c.After {
				return false
			}
		}
	}
	return true
}
func (s *Service) reconcile(ctx context.Context, j Job, p Preview) bool {
	account, err := s.mal.Account(ctx)
	if err != nil || account != p.AccountID {
		return false
	}
	remote, err := s.mal.Read(ctx, p.Resolution.MALID)
	if err != nil || !patchObserved(p, remote) {
		return false
	}
	return s.finish(ctx, j, &p, "synced", "", "Recovered by observing the intended MAL state; no update was resubmitted.", Observation{State: remote, Source: "reconciliation", At: s.now()}) == nil
}
func (s *Service) execute(ctx context.Context, j Job) error {
	var sourceExists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM plex_payload WHERE id=?`, j.EventID).Scan(&sourceExists); err != nil {
		if err == sql.ErrNoRows {
			_, err = s.db.ExecContext(ctx, `DELETE FROM sync_job WHERE id=?`, j.ID)
			return err
		}
		return err
	}
	in, _, err := s.input(ctx, j.EventID)
	if err != nil { // Deletion cancels unsent work.
		return s.finish(ctx, j, nil, "cancelled", "EVENT_DELETED", "The source event was deleted.")
	}
	if !s.writes {
		return s.finish(ctx, j, nil, "needs_review", "WRITES_DISABLED", "MAL writes are disabled on this instance.")
	}
	if _, err = s.currentEligibility(ctx, in); err != nil {
		typed := err.(*Error)
		return s.finish(ctx, j, nil, "needs_review", typed.Code, typed.Message)
	}
	if err = s.checkUncertain(ctx, j.EventID); err != nil {
		return s.finish(ctx, j, nil, "needs_review", "UNCERTAIN_WRITE", err.Error())
	}
	var p Preview
	if j.Kind == "live" {
		p = s.evaluateLive(ctx, in)
		if p.Ready && j.AccountID != "" && j.AccountID != p.AccountID {
			p.Ready = false
			p.Code = "ACCOUNT_CHANGED"
			p.Message = "The MAL account changed after intake. Create a fresh preview."
		}
		if p.Ready && j.AccountID == "" {
			p.Ready = false
			p.Code = "MAL_AUTH_FAILED"
			p.Message = "The destination MAL account could not be bound at intake. Create a fresh preview."
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO sync_preview(id,event_id,payload,created_at) VALUES(?,?,?,?)`, p.ID, j.EventID, marshal(p), p.CreatedAt)
		if err != nil {
			return err
		}
		j.PreviewID = p.ID
	} else {
		err = readJSON(ctx, s.db, `SELECT payload FROM sync_preview WHERE id=?`, &p, j.PreviewID)
		if err != nil {
			return err
		}
		fresh := s.evaluate(ctx, in, p.Correction)
		if !s.now().Before(p.ExpiresAt) || !samePreview(p, fresh) {
			return s.finish(ctx, j, &p, "needs_review", "STALE_PREVIEW", "The preview expired or its account, mapping, or MAL state changed. Create a fresh preview.")
		}
	}
	if !p.Ready {
		return s.finish(ctx, j, &p, statusFor(p.Code), p.Code, p.Message)
	}
	if len(p.Changes) == 0 {
		return s.finish(ctx, j, &p, "no_change", "", "MAL already has this progress or rating. No update was sent.", Observation{State: *p.Before, Source: "no_change_read", At: s.now()})
	}
	if _, err = s.currentEligibility(ctx, in); err != nil {
		typed := err.(*Error)
		return s.finish(ctx, j, &p, "needs_review", typed.Code, typed.Message)
	}
	j.Status = "processing"
	j.UpdatedAt = s.now()
	if err = s.putJob(ctx, j); err != nil {
		return err
	} // persist absolute intent before sending
	remote, err := s.mal.Write(ctx, p.AccountID, p.Resolution.MALID, p.Changes)
	if err != nil {
		if typed, ok := err.(*Error); ok {
			return s.finish(context.Background(), j, &p, statusFor(typed.Code), typed.Code, typed.Message)
		}
		return s.finish(context.Background(), j, &p, "needs_review", "UNCERTAIN_WRITE", "MAL did not confirm the update. Retry first reconciles its state; the request will not be blindly repeated.")
	}
	if !patchObserved(p, remote) {
		return s.finish(context.Background(), j, &p, "needs_review", "UNCERTAIN_WRITE", "MAL returned a state different from the saved intent. Review before retrying.", Observation{State: remote, Source: writeEvidenceSource(remote), At: s.now()})
	}
	return s.finish(context.Background(), j, &p, "synced", "", "MyAnimeList updated successfully.", Observation{State: remote, Source: writeEvidenceSource(remote), At: s.now()})
}

type Observation struct {
	State  MALState
	Source string
	At     time.Time
}

func (s *Service) finish(ctx context.Context, j Job, p *Preview, status, code, message string, observations ...Observation) error {
	j.Status = status
	j.Code = code
	j.Message = message
	j.UpdatedAt = s.now()
	a := Attempt{ID: id(), JobID: j.ID, Kind: j.Kind, Status: status, CreatedAt: j.CreatedAt, FinishedAt: j.UpdatedAt, Preview: p, Code: code, Message: message}
	if len(observations) > 0 {
		observation := observations[0]
		state := observation.State
		stamp := observation.At
		a.Observed = &state
		a.ObservedAt = &stamp
		a.ObservationSource = observation.Source
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT 1 FROM plex_payload WHERE id=?`, j.EventID).Scan(&exists); err != nil {
		return err
	}
	var priorStatus string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM sync_job WHERE id=?`, j.ID).Scan(&priorStatus); err != nil {
		return err
	}
	if priorStatus == status && status != "queued" && status != "processing" {
		return nil
	}
	if status == "synced" && a.Observed == nil {
		return fail("INVALID_OUTCOME", "A successful sync requires an observed MAL result.")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sync_attempt(id,job_id,event_id,payload,created_at) VALUES(?,?,?,?,?)`, a.ID, j.ID, j.EventID, marshal(a), j.UpdatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE sync_job SET status=?,payload=?,preview_id=NULLIF(?,'') WHERE id=?`, j.Status, marshal(j), j.PreviewID, j.ID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// Compatible history and notifications use the same resolved outcome, never
	// the original mutable input. The durable attempt remains authoritative.
	if s.storeHistory == nil {
		return nil
	}
	in, _, err := s.input(ctx, j.EventID)
	if err != nil {
		return err
	}
	au := &domain.AnimeUpdate{PlexId: in.EventID, SourceDB: domain.PlexSupportedDBs(in.Source), SourceId: in.SourceID, SeasonNum: in.Season, EpisodeNum: in.Episode, Timestamp: j.UpdatedAt, Status: domain.AnimeUpdateStatusFailed, ErrorType: domain.AnimeUpdateErrorType(code), ErrorMessage: message, ListDetails: domain.ListDetails{Title: in.Title}}
	au.Plex = &domain.Plex{ID: in.EventID, Event: domain.PlexEvent(in.Event)}
	au.Plex.Account.Title = in.Account
	au.Plex.Metadata.LibrarySectionTitle = in.Library
	au.Plex.Metadata.Type = domain.PlexMediaType(in.MediaType)
	if p != nil {
		au.MALId = p.Resolution.MALID
		au.EpisodeNum = p.Resolution.Episode
		// A failed write may have no after-state at all. Use its actual
		// preflight read, never the planned patch, in compatible history.
		b := a.Observed
		if b == nil {
			b = p.Before
		}
		if b != nil {
			title, picture, total := b.Title, b.PictureURL, b.TotalEpisodes
			if p.Before != nil {
				if title == "" {
					title = p.Before.Title
				}
				if picture == "" {
					picture = p.Before.PictureURL
				}
				if total == 0 {
					total = p.Before.TotalEpisodes
				}
			}
			au.ListDetails = domain.ListDetails{Title: title, PictureURL: picture, TotalEpisodeNum: total, WatchedNum: b.EpisodesWatched, Status: mal.AnimeStatus(b.Status), RewatchNum: b.RewatchCount}
			au.ListStatus = mal.AnimeListStatus{Status: mal.AnimeStatus(b.Status), NumEpisodesWatched: b.EpisodesWatched, Score: b.Score, IsRewatching: b.IsRewatching, NumTimesRewatched: b.RewatchCount, StartDate: b.StartDate, FinishDate: b.FinishDate, UpdatedAt: b.UpdatedAt}
		}
	}
	success := status == "synced" || status == "no_change"
	if success {
		au.Status = domain.AnimeUpdateStatusSuccess
		au.ErrorMessage = ""
	}
	if err = s.storeHistory(ctx, au); err == nil && s.publish != nil {
		// Publication follows one committed logical completion. No-op and
		// recovered results are evidence updates, not another success event.
		changedTerminal := priorStatus != status
		confirmedWrite := status == "synced" && (a.ObservationSource == "write_response" || a.ObservationSource == "confirmation_read")
		if changedTerminal && (!success || confirmedWrite) {
			s.publish(au, success)
		}
	}

	return err
}

// WithDeletion holds the writer exclusion until deleting queued work and source
// rows commits. A completed delete can never race a later external write.
func (s *Service) WithDeletion(ctx context.Context, fn func() error) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	if err := fn(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sync_job WHERE event_id NOT IN (SELECT id FROM plex_payload);DELETE FROM sync_input WHERE event_id NOT IN (SELECT id FROM plex_payload);DELETE FROM sync_preview WHERE event_id NOT IN (SELECT id FROM plex_payload);DELETE FROM sync_attempt WHERE event_id NOT IN (SELECT id FROM plex_payload);`)
	return err
}

func writeEvidenceSource(state MALState) string {
	if state.EvidenceSource == "confirmation_read" {
		return "confirmation_read"
	}
	return "write_response"
}
