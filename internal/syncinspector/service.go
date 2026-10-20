package syncinspector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/shinkro/shinkro/internal/domain"
)

type Maps interface {
	Snapshot(context.Context) (*domain.AnimeMap, *domain.MapSettings, error)
	Refresh(context.Context) error
}
type MAL interface {
	Account(context.Context) (string, error)
	Read(context.Context, int) (MALState, error)
	Write(context.Context, string, int, []Change) (MALState, error)
}
type Service struct {
	eligibility  Eligibility
	db           *sql.DB
	maps         Maps
	mal          MAL
	refresh      func(context.Context) error
	storeHistory func(context.Context, *domain.AnimeUpdate) error
	publish      func(*domain.AnimeUpdate, bool)
	writes       bool
	writer       sync.Mutex
	wake         chan struct{}
	stop         chan struct{}
	done         chan struct{}
	now          func() time.Time
}

func New(db *sql.DB, maps Maps, mal MAL, writes bool, refresh func(context.Context) error, store func(context.Context, *domain.AnimeUpdate) error, publish func(*domain.AnimeUpdate, bool)) *Service {
	return &Service{db: db, maps: maps, mal: mal, writes: writes, refresh: refresh, storeHistory: store, publish: publish, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) WritesEnabled() bool { return s.writes }
func (s *Service) Start()              { go s.run() }
func (s *Service) Close()              { close(s.stop); <-s.done }
func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) Refresh(ctx context.Context) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	if err := s.refresh(ctx); err != nil {
		return fail("SOURCE_UNAVAILABLE", "Catalogue refresh failed; the previous catalogue is retained.")
	}
	if err := s.maps.Refresh(ctx); err != nil {
		return fail("SOURCE_UNAVAILABLE", "Mapping refresh failed; the previous validated mapping snapshot is retained.")
	}
	return s.markSourcesRefreshed(ctx)
}
func inputFromAnime(a *domain.AnimeUpdate, event domain.PlexEvent) Input {
	p := a.Plex
	in := Input{Version: 1, EventID: a.PlexId, Source: string(a.SourceDB), SourceID: a.SourceId, Season: a.SeasonNum, Episode: a.EpisodeNum, Event: string(event), ReceivedAt: a.Timestamp}
	if p != nil {
		in.Title = a.ListDetails.Title
		in.EpisodeTitle = p.Metadata.Title
		in.Account = p.Account.Title
		in.Library = p.Metadata.LibrarySectionTitle
		in.MediaType = string(p.Metadata.Type)
		in.Rating = p.Rating
		in.ServerID = p.Server.UUID
		in.ReceivedAt = p.TimeStamp
	}
	return in
}

// Enqueue is called synchronously at intake; no goroutine can lose accepted work.
func (s *Service) Enqueue(ctx context.Context, a *domain.AnimeUpdate, event domain.PlexEvent) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	in := inputFromAnime(a, event)
	var sourceExists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM plex_payload WHERE id=?`, in.EventID).Scan(&sourceExists); err != nil {
		return err
	}
	var existing int
	if s.db.QueryRowContext(ctx, `SELECT 1 FROM sync_job WHERE id=?`, "live-"+fmt.Sprint(in.EventID)).Scan(&existing) == nil {
		return nil
	}
	var captured Input
	if err := readJSON(ctx, s.db, `SELECT input FROM sync_input WHERE event_id=?`, &captured, in.EventID); err == nil {
		in = captured
	} else if err != sql.ErrNoRows {
		return err
	}
	if _, err := s.currentEligibility(ctx, in); err != nil {
		return err
	}
	account, _ := s.mal.Account(ctx)
	now := s.now()
	j := Job{AccountID: account, ID: "live-" + fmt.Sprint(in.EventID), EventID: in.EventID, Kind: "live", Status: "queued", CreatedAt: now, UpdatedAt: now}
	duplicateOf, err := s.recentDuplicate(ctx, in, account, now)
	if err != nil {
		return err
	}
	if duplicateOf > 0 {
		j.Status = "no_change"
		j.Code = "DUPLICATE_SOURCE_EVENT"
		j.Message = fmt.Sprintf("Duplicate source event received within 30 seconds of event %d. No additional MAL update was sent.", duplicateOf)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO sync_input(event_id,input) VALUES(?,?)`, in.EventID, marshal(in)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO sync_job(id,event_id,kind,status,payload,created_at) VALUES(?,?,?,?,?,?)`, j.ID, j.EventID, j.Kind, j.Status, marshal(j), now); err != nil {
		return err
	}
	if duplicateOf > 0 {
		attempt := Attempt{ID: id(), JobID: j.ID, Kind: j.Kind, Status: j.Status, CreatedAt: now, FinishedAt: now, Code: j.Code, Message: j.Message}
		if _, err = tx.ExecContext(ctx, `INSERT INTO sync_attempt(id,job_id,event_id,payload,created_at) VALUES(?,?,?,?,?)`, attempt.ID, j.ID, in.EventID, marshal(attempt), now); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err == nil {
		s.signal()
	}
	return err
}
func (s *Service) input(ctx context.Context, key int64) (Input, bool, error) {
	var in Input
	err := readJSON(ctx, s.db, `SELECT input FROM sync_input WHERE event_id=?`, &in, key)
	if err == nil {
		return in, false, nil
	}
	if err != sql.ErrNoRows {
		return in, false, err
	}
	// Legacy records retain original Plex indexes; never replay mapped history episode.
	var guids string
	var guid string
	var event, media string
	var stamp time.Time
	err = s.db.QueryRowContext(ctx, `SELECT id,COALESCE(grand_parent_title,''),COALESCE(title,''),account_title,library_section_title,event,type,COALESCE(parent_index,0),COALESCE(metadata_index,0),COALESCE(rating,0),time_stamp,COALESCE(guid_string,''),COALESCE(guids,'') FROM plex_payload WHERE id=?`, key).Scan(&in.EventID, &in.Title, &in.EpisodeTitle, &in.Account, &in.Library, &event, &media, &in.Season, &in.Episode, &in.Rating, &stamp, &guid, &guids)
	if err != nil {
		return in, true, err
	}
	in.Event = event
	in.MediaType = media
	in.ReceivedAt = stamp
	if media == "movie" {
		in.Title = in.EpisodeTitle
		in.Season = 1
		in.Episode = 1
	}
	// Extracted provider stored in history is an observation, not a converted MAL episode.
	var src string
	var srcID int
	err = s.db.QueryRowContext(ctx, `SELECT source_db,source_id FROM anime_update WHERE plex_id=? AND source_id>0 ORDER BY time_stamp ASC,id ASC LIMIT 1`, key).Scan(&src, &srcID)
	if err == nil {
		in.Source = src
		in.SourceID = srcID
	} else {
		g := domain.GUID{GUID: guid}
		for _, agent := range []domain.PlexSupportedAgents{domain.HAMA, domain.MALAgent} {
			source, n, e := g.HamaMALAgent(agent)
			if e == nil {
				in.Source = string(source)
				in.SourceID = n
				break
			}
		}
	}
	if in.SourceID > 0 {
		in.Version = 1
	}
	return in, true, nil
}
func eligible(in Input) (bool, string) {
	if in.Source != "myanimelist" && in.Source != "anidb" && in.Source != "tvdb" && in.Source != "tmdb" {
		return false, "The original source provider is unavailable or unsupported."
	}
	if in.Version < 1 || in.SourceID < 1 {
		return false, "The original provider ID is unavailable. This legacy event cannot be safely replayed."
	}
	if in.Event != "media.rate" && in.Event != "media.scrobble" {
		return false, "This event type cannot update MAL."
	}
	if in.MediaType != "movie" && in.MediaType != "episode" {
		return false, "The original media type is unavailable."
	}
	if in.Episode < 1 && in.Event == "media.scrobble" {
		return false, "The original source episode is unavailable."
	}
	return true, ""
}
func (s *Service) Detail(ctx context.Context, key int64) (Detail, error) {
	in, legacy, err := s.input(ctx, key)
	if err != nil {
		return Detail{}, err
	}
	ok, reason := eligible(in)
	e := Event{Input: in, ID: key, Status: "received", Legacy: legacy, LastAction: "Received"}
	var histStatus, code, msg string
	var mid int
	var raw string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(status,''),COALESCE(error_type,''),COALESCE(error_message,''),mal_id,COALESCE(list_details,'{}') FROM anime_update WHERE plex_id=? ORDER BY time_stamp ASC,id ASC LIMIT 1`, key).Scan(&histStatus, &code, &msg, &mid, &raw)
	attempts := []Attempt{}
	if err == nil {
		e.Status = statusFor(code)
		if histStatus == "SUCCESS" {
			e.Status = "synced"
		}
		e.ErrorCode = code
		e.ErrorMessage = safeLegacyMessage(code)
		e.MALID = mid
		var ld domain.ListDetails
		_ = json.Unmarshal([]byte(raw), &ld)
		e.MALTitle = ld.Title
		e.LastAction = "Original sync"
		if legacy {
			attempts = append(attempts, Attempt{ID: "legacy-" + fmt.Sprint(key), Kind: "original", Status: e.Status, CreatedAt: in.ReceivedAt, FinishedAt: in.ReceivedAt, Code: code, Message: e.ErrorMessage})
		}
	} else if err != sql.ErrNoRows {
		return Detail{}, err
	} else {
		var extracted sql.NullBool
		var extractionCode string
		if err = s.db.QueryRowContext(ctx, `SELECT success,COALESCE(error_type,'') FROM plex_payload WHERE id=?`, key).Scan(&extracted, &extractionCode); err != nil {
			return Detail{}, err
		}
		if extracted.Valid && !extracted.Bool {
			e.Status = "failed"
			e.ErrorCode = extractionCode
			e.ErrorMessage = extractionMessage(extractionCode)
			e.LastAction = "Plex extraction"
			attempts = append(attempts, Attempt{ID: "extraction-" + fmt.Sprint(key), Kind: "extraction", Status: "failed", CreatedAt: in.ReceivedAt, FinishedAt: in.ReceivedAt, Code: extractionCode, Message: e.ErrorMessage})
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM sync_attempt WHERE event_id=? ORDER BY created_at,id`, key)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var raw string
		var a Attempt
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return Detail{}, err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			rows.Close()
			return Detail{}, err
		}
		attempts = append(attempts, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Detail{}, err
	}
	original := e
	if !legacy {
		for _, a := range attempts {
			if a.Kind == "live" {
				original.Status = a.Status
				original.ErrorCode = a.Code
				original.ErrorMessage = a.Message
				original.LastAction = "Original sync"
				if a.Preview != nil {
					original.MALID = a.Preview.Resolution.MALID
					original.MALTitle = a.Preview.Resolution.Title
				}
				break
			}
		}
	}
	var j Job
	if err = readJSON(ctx, s.db, `SELECT payload FROM sync_job WHERE event_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, &j, key); err == nil {
		e.Status = j.Status
		e.ErrorCode = j.Code
		e.ErrorMessage = j.Message
		e.LastAction = j.Kind
		if len(attempts) > 0 {
			a := attempts[len(attempts)-1]
			if a.Preview != nil {
				e.MALID = a.Preview.Resolution.MALID
				if a.Preview.Before != nil {
					e.MALTitle = a.Preview.Before.Title
				}
			}
		}
	}
	overrides, err := s.Overrides(ctx)
	if err != nil {
		return Detail{}, err
	}
	scoped := []Override{}
	for _, o := range overrides {
		if scope(o, in) {
			scoped = append(scoped, o)
		}
	}
	return Detail{Original: &original, Event: e, Input: in, Attempts: attempts, Legacy: legacy, ReplayEligible: ok, ReplayReason: reason, Overrides: scoped}, nil
}
func statusFor(code string) string {
	switch code {
	case "ANIME_NOT_IN_DB", "MAPPING_NOT_FOUND":
		return "needs_mapping"
	case "MAL_AUTH_FAILED", "ACCOUNT_CHANGED":
		return "needs_sign_in"
	case "AMBIGUOUS_MAPPING", "STALE_PREVIEW", "EVENT_NOT_ELIGIBLE", "ELIGIBILITY_UNAVAILABLE", "STALE_RATING", "STALE_WATCH_CYCLE", "LEGACY_INPUT", "INVALID_EPISODE", "UNDEFINED_EPISODE", "UNCERTAIN_WRITE", "WRITES_DISABLED":
		return "needs_review"
	default:
		return "failed"
	}
}
func safeLegacyMessage(code string) string {
	switch code {
	case "AGENT_NOT_SUPPORTED", "EXTRACTION_FAILED":
		return extractionMessage(code)
	case "UNKNOWN_ERROR":
		return "This event could not be prepared for sync. Inspect its source details before retrying."
	case "ANIME_NOT_IN_DB":
		return "No MAL destination was found in the catalogue."
	case "MAPPING_NOT_FOUND":
		return "No matching community rule was found."
	case "MAL_AUTH_FAILED":
		return "Reconnect your MyAnimeList account."
	case "MAL_API_FETCH_FAILED":
		return "MyAnimeList state could not be read."
	case "MAL_API_UPDATE_FAILED":
		return "MyAnimeList update failed. Preview current state before retrying."
	}
	if code != "" {
		return "The original sync failed. Preview the event to evaluate it using current data."
	}
	return ""
}

type actorKey struct{}

func WithActor(ctx context.Context, id int) context.Context {
	return context.WithValue(ctx, actorKey{}, id)
}
func actor(ctx context.Context) int { v, _ := ctx.Value(actorKey{}).(int); return v }

func extractionMessage(code string) string {
	switch code {
	case "AGENT_NOT_SUPPORTED":
		return "Plex metadata uses an unsupported agent. No MAL sync was attempted."
	case "EXTRACTION_FAILED":
		return "Plex could not determine the source provider ID. No MAL sync was attempted."
	default:
		return "Plex extraction failed before any MAL sync was attempted."
	}
}
