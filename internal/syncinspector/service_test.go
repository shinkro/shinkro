package syncinspector

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type fakeMaps struct {
	m   *domain.AnimeMap
	err error
}

func (f *fakeMaps) Snapshot(context.Context) (*domain.AnimeMap, *domain.MapSettings, error) {
	return f.m, &domain.MapSettings{}, f.err
}
func (f *fakeMaps) Refresh(context.Context) error { return f.err }

type fakeMAL struct {
	mu             sync.Mutex
	account        string
	state          MALState
	writes         int
	failWrite      bool
	applyOnFailure bool
	entered        chan struct{}
	release        chan struct{}
}

func (f *fakeMAL) Account(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.account, nil
}
func (f *fakeMAL) Read(_ context.Context, mid int) (MALState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v := f.state
	v.MALID = mid
	return v, nil
}
func (f *fakeMAL) Write(ctx context.Context, account string, mid int, changes []Change) (MALState, error) {
	f.mu.Lock()
	if account != f.account {
		f.mu.Unlock()
		return MALState{}, errors.New("account changed")
	}
	f.writes++
	entered, release := f.entered, f.release
	f.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-ctx.Done():
			return MALState{}, ctx.Err()
		case <-release:
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.failWrite || f.applyOnFailure {
		for _, c := range changes {
			switch c.Field {
			case "episodesWatched":
				f.state.EpisodesWatched = asInt(c.After)
			case "score":
				f.state.Score = asInt(c.After)
			case "status":
				f.state.Status = c.After.(string)
			case "startDate":
				f.state.StartDate = c.After.(string)
			case "finishDate":
				f.state.FinishDate = c.After.(string)
			case "rewatchCount":
				f.state.RewatchCount = asInt(c.After)
			case "isRewatching":
				f.state.IsRewatching = c.After.(bool)
			}
		}
		f.state.UpdatedAt = time.Now().UTC()
	}
	if f.failWrite {
		return MALState{}, errors.New("response lost")
	}
	v := f.state
	v.MALID = mid
	return v, nil
}
func fixture(t *testing.T, writes bool) (*Service, *sql.DB, *fakeMAL, *fakeMaps, context.Context) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db")+"?_pragma=busy_timeout%3d1000&_pragma=foreign_keys%3d1")
	require.NoError(t, err)
	_, err = db.Exec(`
 CREATE TABLE plex_payload(id INTEGER PRIMARY KEY,grand_parent_title TEXT,title TEXT,account_title TEXT,library_section_title TEXT,event TEXT,type TEXT,parent_index INTEGER,metadata_index INTEGER,rating REAL,time_stamp TIMESTAMP,guid_string TEXT,guids TEXT,success BOOLEAN,error_type TEXT,error_msg TEXT);
 CREATE TABLE anime_update(id INTEGER PRIMARY KEY,plex_id INTEGER,mal_id INTEGER,source_db TEXT,source_id INTEGER,episode_num INTEGER,season_num INTEGER,time_stamp TIMESTAMP,list_details TEXT,status TEXT,error_type TEXT,error_message TEXT);
 CREATE TABLE anime(mal_id INTEGER PRIMARY KEY,anidb_id INTEGER,tvdb_id INTEGER,tmdb_id INTEGER,title TEXT,en_title TEXT,type TEXT);
 ` + Schema)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	maps := &fakeMaps{m: &domain.AnimeMap{AnimeTVShows: &domain.AnimeTVShows{Anime: []domain.AnimeTV{{Malid: 395, Tvdbid: 78916, TvdbSeason: 2, Start: 1}}}, AnimeMovies: &domain.AnimeMovies{}}}
	remote := &fakeMAL{account: "42", state: MALState{Title: "Fixture anime", TotalEpisodes: 24, Status: "watching", EpisodesWatched: 0}}
	svc := New(db, maps, remote, writes, func(context.Context) error { return nil }, nil, nil)
	svc.SetEligibility(func(context.Context, Input) ([]string, error) { return nil, nil })
	return svc, db, remote, maps, WithActor(context.Background(), 1)
}
func seed(t *testing.T, s *Service, in Input) {
	t.Helper()
	if in.Version == 0 {
		in.Version = 1
	}
	if in.EventID == 0 {
		in.EventID = 1
	}
	if in.Event == "" {
		in.Event = "media.scrobble"
	}
	if in.MediaType == "" {
		in.MediaType = "episode"
	}
	if in.Title == "" {
		in.Title = "Gantz"
	}
	if in.Account == "" {
		in.Account = "Plex user"
	}
	if in.Library == "" {
		in.Library = "Anime"
	}
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = s.now()
	}
	_, err := s.db.Exec(`INSERT INTO plex_payload(id,grand_parent_title,title,account_title,library_section_title,event,type,parent_index,metadata_index,rating,time_stamp,guid_string,guids) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, in.EventID, in.Title, in.EpisodeTitle, in.Account, in.Library, in.Event, in.MediaType, in.Season, in.Episode, in.Rating, in.ReceivedAt, "", "[]")
	require.NoError(t, err)
	_, err = s.db.Exec(`INSERT INTO sync_input(event_id,input) VALUES(?,?)`, in.EventID, marshal(in))
	require.NoError(t, err)
}
func direct(id int64, ep int) Input {
	return Input{Version: 1, EventID: id, Source: "myanimelist", SourceID: 384, Season: 1, Episode: ep, ServerID: "server-A"}
}
func waitJob(t *testing.T, s *Service, key string, status string) Job {
	t.Helper()
	var j Job
	require.Eventually(t, func() bool {
		var err error
		j, err = s.Job(context.Background(), key)
		return err == nil && j.Status == status
	}, 5*time.Second, 10*time.Millisecond)
	return j
}
func TestCanonicalRoutingAndGantz(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	for _, v := range []struct{ aid, mid int }{{2400, 384}, {19908, 63509}, {20040, 64131}, {19889, 63468}, {13514, 36552}, {19859, 63367}} {
		_, err := db.Exec(`INSERT INTO anime(mal_id,anidb_id,tvdb_id,title,type) VALUES(?,?,?,?,?)`, v.mid, v.aid, 78916, "Canonical title", "ona")
		require.NoError(t, err)
	}
	for n, v := range []struct{ aid, mid int }{{2400, 384}, {19908, 63509}, {20040, 64131}, {19889, 63468}, {13514, 36552}, {19859, 63367}} {
		key := int64(n + 1)
		seed(t, s, Input{EventID: key, Source: "anidb", SourceID: v.aid, Season: 1, Episode: 2})
		p, err := s.Preview(ctx, key, nil)
		require.NoError(t, err)
		require.True(t, p.Ready, p.Message)
		require.Equal(t, v.mid, p.Resolution.MALID)
		require.Equal(t, "canonical_anidb", p.Resolution.Origin)
	}
	seed(t, s, Input{EventID: 20, Source: "anidb", SourceID: 2400, Season: 2, Episode: 1})
	p, err := s.Preview(ctx, 20, nil)
	require.NoError(t, err)
	require.True(t, p.Ready, p.Message)
	require.Equal(t, 395, p.Resolution.MALID)
	require.Zero(t, remote.writes)
	// Preserve the existing exactly-one-TV tie break; multiple TV entries block.
	_, err = db.Exec(`INSERT INTO anime(mal_id,anidb_id,title,type) VALUES(99,2400,'TV candidate','tv')`)
	require.NoError(t, err)
	mid, _, _, err := s.catalogue(ctx, "anidb", 2400)
	require.NoError(t, err)
	require.Equal(t, 99, mid)
	_, err = db.Exec(`INSERT INTO anime(mal_id,anidb_id,title,type) VALUES(100,2400,'Other TV','tv')`)
	require.NoError(t, err)
	_, _, _, err = s.catalogue(ctx, "anidb", 2400)
	require.Equal(t, "AMBIGUOUS_MAPPING", err.(*Error).Code)
}
func TestBeerusMissingLinkRecoveryKeepsOriginal(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, false)
	seed(t, s, Input{Source: "anidb", SourceID: 19859, Season: 1, Episode: 2})
	_, err := db.Exec(`INSERT INTO anime_update(plex_id,mal_id,source_db,source_id,episode_num,time_stamp,list_details,status,error_type,error_message) VALUES(1,0,'anidb',19859,2,?,'{}','FAILED','ANIME_NOT_IN_DB','secret token=bad')`, s.now())
	require.NoError(t, err)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.False(t, p.Ready)
	require.Equal(t, "MAPPING_NOT_FOUND", p.Code)
	_, err = db.Exec(`INSERT INTO anime(mal_id,anidb_id,title,type) VALUES(63367,19859,'Dragon Ball Super: Beerus','tv')`)
	require.NoError(t, err)
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready, p.Message)
	require.Equal(t, 63367, p.Resolution.MALID)
	require.False(t, p.WritesEnabled)
	require.Zero(t, remote.writes)
	d, err := s.Detail(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ANIME_NOT_IN_DB", d.Original.ErrorCode)
	require.NotContains(t, marshal(d), "token=bad")
	require.Len(t, d.Attempts, 0) // captured event, original durable trace not fabricated
}
func TestPlannerSafety(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	in := Input{Event: "media.scrobble", ReceivedAt: now.Add(-24 * time.Hour)}
	before := MALState{EpisodesWatched: 8, TotalEpisodes: 12, Status: "watching", Score: 9, StartDate: "2026-01-01", RewatchCount: 2}
	after, changes, err := plan(in, Resolution{Episode: 6}, before, now)
	require.NoError(t, err)
	require.Empty(t, changes)
	require.Equal(t, before, after)
	after, changes, err = plan(in, Resolution{Episode: 12}, before, now)
	require.NoError(t, err)
	require.Equal(t, "2026-10-19", after.FinishDate)
	require.Equal(t, 9, after.Score)
	require.Equal(t, "2026-01-01", after.StartDate)
	require.Equal(t, 2, after.RewatchCount)
	require.NotEmpty(t, changes)
	_, _, err = plan(in, Resolution{Episode: 13}, before, now)
	require.Equal(t, "INVALID_EPISODE", err.(*Error).Code)
	in.Event = "media.rate"
	in.Rating = 0
	before.UpdatedAt = in.ReceivedAt.Add(time.Minute)
	_, _, err = plan(in, Resolution{}, before, now)
	require.Equal(t, "STALE_RATING", err.(*Error).Code)
	before.UpdatedAt = time.Time{}
	after, changes, err = plan(in, Resolution{}, before, now)
	require.NoError(t, err)
	require.Zero(t, after.Score)
	require.Len(t, changes, 1)
}
func TestPreviewNoWritesAndStaleApproval(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 2))
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready)
	for i := 0; i < 3; i++ {
		_, err = s.Preview(ctx, 1, nil)
		require.NoError(t, err)
	}
	require.Zero(t, remote.writes)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM anime_update`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sync_attempt`).Scan(&count))
	require.Zero(t, count)
	remote.state.EpisodesWatched = 3
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "STALE_PREVIEW", err.(*Error).Code)
	remote.state.EpisodesWatched = 0
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	remote.account = "99"
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "STALE_PREVIEW", err.(*Error).Code)
	remote.account = "42"
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	s.now = func() time.Time { return p.ExpiresAt.Add(time.Second) }
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "STALE_PREVIEW", err.(*Error).Code)
}
func TestReplayDedupAndDurableRestart(t *testing.T) {
	s, _, remote, maps, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	same, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, j.ID, same.ID)
	p2, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	same, err = s.Replay(ctx, 1, p2.ID)
	require.NoError(t, err)
	require.Equal(t, j.ID, same.ID)
	restarted := New(s.db, maps, remote, true, func(context.Context) error { return nil }, nil, nil)
	restarted.SetEligibility(s.eligibility)
	restarted.Start()
	defer restarted.Close()
	waitJob(t, restarted, j.ID, "synced")
	remote.mu.Lock()
	require.Equal(t, 1, remote.writes)
	require.Equal(t, 4, remote.state.EpisodesWatched)
	remote.mu.Unlock()
	same, err = restarted.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, j.ID, same.ID)
	d, err := restarted.Detail(ctx, 1)
	require.NoError(t, err)
	require.Len(t, d.Attempts, 1)
	require.Equal(t, 4, d.Attempts[0].Preview.Resolution.Episode)
}
func TestDisabledWritesCoverLiveAndReplay(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, false)
	seed(t, s, direct(1, 2))
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "WRITES_DISABLED", err.(*Error).Code)
	in, _, err := s.input(ctx, 1)
	require.NoError(t, err)
	a := &domain.AnimeUpdate{PlexId: 1, SourceDB: domain.MAL, SourceId: 384, SeasonNum: 1, EpisodeNum: 2, Timestamp: in.ReceivedAt}
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	s.Start()
	defer s.Close()
	waitJob(t, s, "live-1", "needs_review")
	remote.mu.Lock()
	require.Zero(t, remote.writes)
	remote.mu.Unlock()
}
func TestUncertainWriteReconcilesWithoutRepeating(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 3))
	remote.failWrite = true
	remote.applyOnFailure = true
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	s.Start()
	defer s.Close()
	waitJob(t, s, j.ID, "needs_review")
	fresh, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.Empty(t, fresh.Changes)
	j2, err := s.Replay(ctx, 1, fresh.ID)
	require.NoError(t, err)
	waitJob(t, s, j2.ID, "no_change")
	old, err := s.Job(ctx, j.ID)
	require.NoError(t, err)
	require.Equal(t, "synced", old.Status)
	remote.mu.Lock()
	require.Equal(t, 1, remote.writes)
	remote.mu.Unlock()
}
func TestUncertainUnobservedWriteBlocksRetry(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 3))
	remote.failWrite = true
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	s.Start()
	defer s.Close()
	waitJob(t, s, j.ID, "needs_review")
	fresh, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	_, err = s.Replay(ctx, 1, fresh.ID)
	require.Equal(t, "UNCERTAIN_WRITE", err.(*Error).Code)
	remote.mu.Lock()
	require.Equal(t, 1, remote.writes)
	remote.mu.Unlock()
}
func TestDeletionWaitsForWriterAndCancelsQueuedWork(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 3))
	remote.entered = make(chan struct{})
	remote.release = make(chan struct{})
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	s.Start()
	defer s.Close()
	select {
	case <-remote.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not send")
	}
	deleted := make(chan error, 1)
	go func() {
		deleted <- s.WithDeletion(ctx, func() error { _, err := db.Exec(`DELETE FROM plex_payload WHERE id=1`); return err })
	}()
	select {
	case <-deleted:
		t.Fatal("deletion raced an active writer")
	case <-time.After(30 * time.Millisecond):
	}
	close(remote.release)
	require.NoError(t, <-deleted)
	_, err = s.Job(ctx, j.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	remote.mu.Lock()
	require.Equal(t, 1, remote.writes)
	remote.mu.Unlock()
}
func TestOriginalEpisodeAndLegacyCapabilities(t *testing.T) {
	s, db, _, _, ctx := fixture(t, true)
	now := s.now()
	_, err := db.Exec(`INSERT INTO plex_payload VALUES(1,'Gantz','ep','u','Anime','media.scrobble','episode',2,13,0,?,'','[]',1,'','')`, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO anime_update(plex_id,mal_id,source_db,source_id,episode_num,season_num,time_stamp,list_details,status,error_type,error_message) VALUES(1,395,'tvdb',78916,1,2,?,'{}','FAILED','MAL_API_UPDATE_FAILED','')`, now)
	require.NoError(t, err)
	d, err := s.Detail(ctx, 1)
	require.NoError(t, err)
	require.True(t, d.Legacy)
	require.True(t, d.ReplayEligible)
	require.Equal(t, 13, d.Input.Episode)
	require.Equal(t, 78916, d.Input.SourceID)
	_, err = db.Exec(`INSERT INTO plex_payload VALUES(2,'Unknown','ep','u','Anime','media.scrobble','episode',1,1,0,?,'plex://episode/notrecoverable','[]',0,'EXTRACTION_FAILED','')`, now)
	require.NoError(t, err)
	d, err = s.Detail(ctx, 2)
	require.NoError(t, err)
	require.False(t, d.ReplayEligible)
	require.NotEmpty(t, d.ReplayReason)
}
func TestRuleImpactScopeRevisionAndInvalidation(t *testing.T) {
	s, db, _, _, ctx := fixture(t, true)
	seed(t, s, Input{EventID: 1, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2, ServerID: "A"})
	seed(t, s, Input{EventID: 2, Source: "anidb", SourceID: 19859, Season: 2, Episode: 2, ServerID: "A"})
	seed(t, s, Input{EventID: 3, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2, ServerID: "B"})
	o := Override{EventID: 1, MALID: 63367, SourceEpisodeStart: 1, SourceEpisodeEnd: 12, MALEpisodeStart: 1}
	impact, err := s.OverrideImpact(ctx, o)
	require.NoError(t, err)
	require.Equal(t, []int64{1}, impact.AffectedEventIDs)
	o.ImpactFingerprint = impact.Fingerprint
	saved, err := s.SaveOverride(ctx, o)
	require.NoError(t, err)
	require.Equal(t, 1, saved.Revision)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready)
	require.Equal(t, "local_override", p.Resolution.Origin)
	require.NoError(t, s.DeleteOverrideRevision(ctx, saved.ID, 1))
	require.Error(t, s.DeleteOverrideRevision(ctx, saved.ID, 1))
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "STALE_PREVIEW", err.(*Error).Code)
	all, err := s.AllOverrides(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.False(t, all[0].Active)
	require.Equal(t, 2, all[0].Revision)
	edited := all[0]
	edited.Active = true
	impact, err = s.OverrideImpact(ctx, edited)
	require.NoError(t, err)
	edited.ImpactFingerprint = impact.Fingerprint
	edited, err = s.UpdateOverride(ctx, edited)
	require.NoError(t, err)
	require.Equal(t, 3, edited.Revision)
	var revisions int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sync_override_revision WHERE id=?`, saved.ID).Scan(&revisions))
	require.Equal(t, 3, revisions)
	o.SourceEpisodeEnd = 30
	_, err = s.OverrideImpact(ctx, o)
	require.Equal(t, "INVALID_EPISODE", err.(*Error).Code)
}
func TestSQLListFiltersAndPagination(t *testing.T) {
	s, _, _, _, ctx := fixture(t, true)
	for n := 1; n <= 6; n++ {
		seed(t, s, direct(int64(n), n))
	}
	rows, count, err := s.List(ctx, 2, 2, "", "", "myanimelist", "media.scrobble")
	require.NoError(t, err)
	require.Equal(t, 6, count)
	require.Len(t, rows, 2)
	require.EqualValues(t, 4, rows[0].ID)
	rows, count, err = s.List(ctx, 25, 0, "Gantz", "received", "", "")
	require.NoError(t, err)
	require.Equal(t, 6, count)
	require.Len(t, rows, 6)
}

func TestFreshLiveRewatchIsSeparateFromReplay(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 1))
	in, _, err := s.input(ctx, 1)
	require.NoError(t, err)
	remote.state = MALState{Title: "Completed title", Status: "completed", EpisodesWatched: 24, TotalEpisodes: 24, RewatchCount: 2}
	replay := s.evaluate(ctx, in, nil)
	require.True(t, replay.Ready)
	require.Empty(t, replay.Changes)
	require.False(t, replay.After.IsRewatching)
	live := s.evaluateLive(ctx, in)
	require.True(t, live.Ready)
	require.True(t, live.After.IsRewatching)
	require.Equal(t, 1, live.After.EpisodesWatched)
	require.Equal(t, 2, live.After.RewatchCount)
	remote.state.IsRewatching = true
	remote.state.EpisodesWatched = 23
	in.Episode = 24
	live = s.evaluateLive(ctx, in)
	require.False(t, live.After.IsRewatching)
	require.Equal(t, 3, live.After.RewatchCount)
	_, err = remote.Write(ctx, "42", 384, live.Changes)
	require.NoError(t, err)
	duplicate := s.evaluateLive(ctx, in)
	require.Empty(t, duplicate.Changes)
	require.Equal(t, 3, duplicate.After.RewatchCount)
}
func TestCrashAfterWriteReconcilesSavedAbsoluteIntent(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	j.Status = "processing"
	require.NoError(t, s.putJob(ctx, j))
	remote.state = *p.After
	s.Start()
	defer s.Close()
	waitJob(t, s, j.ID, "synced")
	remote.mu.Lock()
	require.Zero(t, remote.writes)
	remote.mu.Unlock()
	var attempts int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sync_attempt`).Scan(&attempts))
	require.Equal(t, 1, attempts)
}
func TestProviderUnavailableDoesNotBecomeCatalogueFallback(t *testing.T) {
	s, db, _, maps, ctx := fixture(t, true)
	seed(t, s, Input{Source: "tvdb", SourceID: 123, Season: 1, Episode: 1})
	_, err := db.Exec(`INSERT INTO anime(mal_id,tvdb_id,title,type)VALUES(384,123,'Gantz','tv')`)
	require.NoError(t, err)
	maps.err = errors.New("HTTP 500")
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.False(t, p.Ready)
	require.Equal(t, "SOURCE_UNAVAILABLE", p.Code)
	require.Zero(t, p.Resolution.MALID)
}
func TestStrictMappingConflictsExplicitGapsAndSplitCour(t *testing.T) {
	m := domain.AnimeTVShows{Anime: []domain.AnimeTV{{Malid: 1, Tvdbid: 123, TvdbSeason: 1, Start: 1}, {Malid: 2, Tvdbid: 123, TvdbSeason: 1, Start: 13}}}
	r, code := m.ResolveStrict(123, 1, 13)
	require.Empty(t, code)
	require.Equal(t, 2, r.Malid)
	require.Equal(t, 1, r.CalculateEpNum(13))
	m.Anime = append(m.Anime, domain.AnimeTV{Malid: 3, Tvdbid: 123, TvdbSeason: 1, Start: 13})
	_, code = m.ResolveStrict(123, 1, 13)
	require.Equal(t, "AMBIGUOUS_MAPPING", code)
	m = domain.AnimeTVShows{Anime: []domain.AnimeTV{{Malid: 1, Tvdbid: 123, UseMapping: true, AnimeMapping: []domain.AnimeMapping{{TvdbSeason: 1, Start: 1, MappingType: "explicit", ExplicitEpisodes: map[int]int{1: 2, 3: 4}}}}}}
	_, code = m.ResolveStrict(123, 1, 2)
	require.Equal(t, "UNDEFINED_EPISODE", code)
	r, code = m.ResolveStrict(123, 1, 3)
	require.Empty(t, code)
	require.Equal(t, 4, r.CalculateEpNum(3))
}
func TestNoLegacyJobsCreatedOnStart(t *testing.T) {
	s, db, remote, _, _ := fixture(t, true)
	seed(t, s, direct(1, 5))
	_, err := db.Exec(`DELETE FROM sync_input`)
	require.NoError(t, err)
	s.Start()
	s.Close()
	var jobs int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sync_job`).Scan(&jobs))
	require.Zero(t, jobs)
	require.Zero(t, remote.writes)
}

func TestLateDuplicateIntakeCannotResurrectDeletedEvent(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 2))
	a := &domain.AnimeUpdate{PlexId: 1, SourceDB: domain.MAL, SourceId: 384, SeasonNum: 1, EpisodeNum: 2, Timestamp: s.now()}
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	require.NoError(t, s.WithDeletion(ctx, func() error { _, err := db.Exec(`DELETE FROM plex_payload WHERE id=1`); return err }))
	require.ErrorIs(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent), sql.ErrNoRows)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM sync_job`).Scan(&count))
	require.Zero(t, count)
	require.Zero(t, remote.writes)
}

func TestActiveJobCannotHideDifferentCorrection(t *testing.T) {
	s, _, _, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 2))
	p, err := s.Preview(ctx, 1, &Correction{MALID: 384, Episode: 2})
	require.NoError(t, err)
	_, err = s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	different, err := s.Preview(ctx, 1, &Correction{MALID: 395, Episode: 1})
	require.NoError(t, err)
	_, err = s.Replay(ctx, 1, different.ID)
	require.Equal(t, "JOB_IN_PROGRESS", err.(*Error).Code)
}

func TestDuplicateSourceWindowAcrossDistinctPlexIDs(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	clock := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	a := &domain.AnimeUpdate{PlexId: 1, SourceDB: domain.MAL, SourceId: 384, SeasonNum: 1, EpisodeNum: 1, Timestamp: clock}
	for n := int64(1); n <= 3; n++ {
		seed(t, s, direct(n, 1))
	}
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	first, err := s.Job(ctx, "live-1")
	require.NoError(t, err)
	require.Equal(t, "queued", first.Status)
	clock = clock.Add(20 * time.Second)
	a.PlexId = 2
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	duplicate, err := s.Job(ctx, "live-2")
	require.NoError(t, err)
	require.Equal(t, "no_change", duplicate.Status)
	require.Equal(t, "DUPLICATE_SOURCE_EVENT", duplicate.Code)
	require.Contains(t, duplicate.Message, "event 1")
	// A duplicate does not extend the window indefinitely.
	clock = clock.Add(11 * time.Second)
	a.PlexId = 3
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	later, err := s.Job(ctx, "live-3")
	require.NoError(t, err)
	require.Equal(t, "queued", later.Status)
	require.Zero(t, remote.writes)
}
func TestReplayCannotAdvanceAnUnknownNewRewatchCycle(t *testing.T) {
	now := time.Now().UTC()
	in := Input{Event: "media.scrobble", ReceivedAt: now.Add(-time.Hour)}
	before := MALState{Status: "completed", IsRewatching: true, EpisodesWatched: 2, TotalEpisodes: 12, UpdatedAt: now.Add(-time.Minute)}
	after, changes, err := plan(in, Resolution{Episode: 8}, before, now)
	require.Equal(t, "STALE_WATCH_CYCLE", err.(*Error).Code)
	require.Equal(t, before, after)
	require.Empty(t, changes)
	in.ReceivedAt = now
	after, _, err = plan(in, Resolution{Episode: 8}, before, now)
	require.NoError(t, err)
	require.Equal(t, 8, after.EpisodesWatched)
}

func TestUnusedPreviewRetentionKeepsDurableIntents(t *testing.T) {
	s, db, _, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 2))
	old := s.now().Add(-48 * time.Hour)
	s.now = func() time.Time { return old }
	unused, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	used, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, used.ID)
	require.NoError(t, err)
	require.NoError(t, s.prunePreviews(ctx, old.Add(48*time.Hour)))
	var raw string
	require.ErrorIs(t, db.QueryRow(`SELECT payload FROM sync_preview WHERE id=?`, unused.ID).Scan(&raw), sql.ErrNoRows)
	require.NoError(t, db.QueryRow(`SELECT payload FROM sync_preview WHERE id=?`, used.ID).Scan(&raw))
	_, err = s.Job(ctx, j.ID)
	require.NoError(t, err)
}

func TestLiveEightAndReplaySixNeverRegressProgress(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	clock := time.Now().UTC()
	s.now = func() time.Time { return clock }
	seed(t, s, direct(1, 8))
	a := &domain.AnimeUpdate{PlexId: 1, SourceDB: domain.MAL, SourceId: 384, SeasonNum: 1, EpisodeNum: 8, Timestamp: clock}
	require.NoError(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	clock = clock.Add(time.Second)
	seed(t, s, direct(2, 6))
	p, err := s.Preview(ctx, 2, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 2, p.ID)
	require.NoError(t, err)
	s.Start()
	defer s.Close()
	waitJob(t, s, "live-1", "synced")
	blocked := waitJob(t, s, j.ID, "needs_review")
	require.Equal(t, "STALE_PREVIEW", blocked.Code)
	remote.mu.Lock()
	require.Equal(t, 8, remote.state.EpisodesWatched)
	require.Equal(t, 1, remote.writes)
	remote.mu.Unlock()
}

func TestLegacyRuleRequiresExplicitFutureServerScope(t *testing.T) {
	s, _, _, _, ctx := fixture(t, true)
	seed(t, s, Input{EventID: 1, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2})
	seed(t, s, Input{EventID: 2, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2, ServerID: "future-server"})
	seed(t, s, Input{EventID: 3, Source: "anidb", SourceID: 19859, Season: 1, Episode: 2, ServerID: "future-server", Library: "Different library"})
	o := Override{EventID: 1, MALID: 63367, SourceEpisodeStart: 1, SourceEpisodeEnd: 12, MALEpisodeStart: 1}
	impact, err := s.OverrideImpact(ctx, o)
	require.NoError(t, err)
	require.False(t, impact.Ready)
	require.NotEmpty(t, impact.Warnings)
	require.Empty(t, impact.Fingerprint)
	_, err = s.SaveOverride(ctx, o)
	require.Equal(t, "SCOPE_REQUIRED", err.(*Error).Code)
	o.AnyServer = true
	impact, err = s.OverrideImpact(ctx, o)
	require.NoError(t, err)
	require.True(t, impact.Ready)
	require.True(t, impact.Scope.AnyServer)
	require.Empty(t, impact.Scope.ServerID)
	require.Equal(t, []int64{1, 2}, impact.AffectedEventIDs)
	o.ImpactFingerprint = impact.Fingerprint
	saved, err := s.SaveOverride(ctx, o)
	require.NoError(t, err)
	require.True(t, saved.AnyServer)
	current, err := s.Preview(ctx, 2, nil)
	require.NoError(t, err)
	require.True(t, current.Ready, current.Message)
	require.Equal(t, "local_override", current.Resolution.Origin)
	require.Equal(t, 63367, current.Resolution.MALID)
	other, err := s.Preview(ctx, 3, nil)
	require.NoError(t, err)
	require.False(t, other.Ready)
	require.NotEqual(t, "local_override", other.Resolution.Origin)
}
func TestAllServerAndSpecificRuleConflictsAreSymmetric(t *testing.T) {
	s, _, _, _, ctx := fixture(t, true)
	for n, server := range []string{"", "A", "B"} {
		seed(t, s, Input{EventID: int64(n + 1), Source: "anidb", SourceID: 19859, Season: 1, Episode: 2, ServerID: server})
	}
	create := func(o Override) Override {
		impact, err := s.OverrideImpact(ctx, o)
		require.NoError(t, err)
		require.True(t, impact.Ready)
		o.ImpactFingerprint = impact.Fingerprint
		saved, err := s.SaveOverride(ctx, o)
		require.NoError(t, err)
		return saved
	}
	broad := create(Override{EventID: 1, AnyServer: true, MALID: 63367, SourceEpisodeStart: 1, SourceEpisodeEnd: 12, MALEpisodeStart: 1})
	specific := Override{EventID: 2, MALID: 63367, SourceEpisodeStart: 1, SourceEpisodeEnd: 12, MALEpisodeStart: 1}
	impact, err := s.OverrideImpact(ctx, specific)
	require.NoError(t, err)
	require.False(t, impact.Ready)
	require.Len(t, impact.Conflicts, 1)
	require.Equal(t, broad.ID, impact.Conflicts[0].ID)
	require.NoError(t, s.DeleteOverrideRevision(ctx, broad.ID, broad.Revision))
	a := create(specific)
	all := Override{EventID: 1, AnyServer: true, MALID: 63367, SourceEpisodeStart: 2, SourceEpisodeEnd: 4, MALEpisodeStart: 1}
	impact, err = s.OverrideImpact(ctx, all)
	require.NoError(t, err)
	require.False(t, impact.Ready)
	require.Len(t, impact.Conflicts, 1)
	require.Equal(t, a.ID, impact.Conflicts[0].ID)
	// Independent known servers may have different rules for the same numbering.
	specific.EventID = 3
	impact, err = s.OverrideImpact(ctx, specific)
	require.NoError(t, err)
	require.True(t, impact.Ready)
	// Broadening a reviewed scope requires reviewing its new impact.
	specific.ImpactFingerprint = impact.Fingerprint
	specific.AnyServer = true
	_, err = s.SaveOverride(ctx, specific)
	require.Equal(t, "RULE_CONFLICT", err.(*Error).Code)
}
