package syncinspector

import (
	"context"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/stretchr/testify/require"
)

type returnedActualState struct {
	*fakeMAL
	wrongProgress bool
}

func (m returnedActualState) Write(ctx context.Context, account string, mid int, changes []Change) (MALState, error) {
	actual, err := m.fakeMAL.Write(ctx, account, mid, changes)
	if err != nil {
		return actual, err
	}
	actual.Score = 10
	actual.RewatchCount = 5
	actual.EvidenceSource = "confirmation_read"
	if m.wrongProgress {
		actual.EpisodesWatched = 1
	}
	return actual, nil
}
func captureOutcomes(s *Service) (*[]domain.AnimeUpdate, *[]bool) {
	history := []domain.AnimeUpdate{}
	notifications := []bool{}
	s.storeHistory = func(_ context.Context, a *domain.AnimeUpdate) error { history = append(history, *a); return nil }
	s.publish = func(_ *domain.AnimeUpdate, success bool) { notifications = append(notifications, success) }
	return &history, &notifications
}
func TestObservedRemoteStateIsSeparateFromIntentAndCompletionIsIdempotent(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	remote.state.EpisodesWatched = 2
	remote.state.Score = 7
	s.mal = returnedActualState{fakeMAL: remote}
	history, notifications := captureOutcomes(s)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.Equal(t, 7, p.After.Score)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	require.NoError(t, s.execute(ctx, j))
	d, err := s.Detail(ctx, 1)
	require.NoError(t, err)
	require.Len(t, d.Attempts, 1)
	attempt := d.Attempts[0]
	require.NotNil(t, attempt.Observed)
	require.NotNil(t, attempt.ObservedAt)
	require.Equal(t, "confirmation_read", attempt.ObservationSource)
	require.Equal(t, 10, attempt.Observed.Score)
	require.Equal(t, 5, attempt.Observed.RewatchCount)
	require.Equal(t, 7, attempt.Preview.After.Score)
	require.Equal(t, 0, attempt.Preview.After.RewatchCount)
	require.Len(t, *history, 1)
	require.Equal(t, 10, (*history)[0].ListStatus.Score)
	require.Equal(t, 5, (*history)[0].ListStatus.NumTimesRewatched)
	require.Equal(t, []bool{true}, *notifications)
	// A duplicated completion cannot materialize or announce a second outcome.
	require.NoError(t, s.finish(ctx, j, &p, "synced", "", "Repeated completion", Observation{State: *attempt.Observed, Source: "confirmation_read", At: s.now()}))
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_attempt WHERE job_id=?`, j.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.Len(t, *history, 1)
	require.Equal(t, []bool{true}, *notifications)
}
func TestFailedWriteNeverMaterializesItsPlannedFields(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 24))
	remote.state.EpisodesWatched = 2
	remote.state.Score = 7
	remote.failWrite = true
	history, notifications := captureOutcomes(s)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.Equal(t, 24, p.After.EpisodesWatched)
	require.Equal(t, "completed", p.After.Status)
	require.NotEmpty(t, p.After.FinishDate)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	require.NoError(t, s.execute(ctx, j))
	d, err := s.Detail(ctx, 1)
	require.NoError(t, err)
	require.Len(t, d.Attempts, 1)
	require.Nil(t, d.Attempts[0].Observed)
	require.Nil(t, d.Attempts[0].ObservedAt)
	require.Empty(t, d.Attempts[0].ObservationSource)
	require.Equal(t, 24, d.Attempts[0].Preview.After.EpisodesWatched)
	require.Len(t, *history, 1)
	actual := (*history)[0]
	require.Equal(t, domain.AnimeUpdateStatusFailed, actual.Status)
	require.Equal(t, 2, actual.ListStatus.NumEpisodesWatched)
	require.Equal(t, 7, actual.ListStatus.Score)
	require.Empty(t, actual.ListStatus.FinishDate)
	require.Equal(t, []bool{false}, *notifications)
}
func TestMismatchingRemoteResultRemainsObservedButNotSuccessful(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	s.mal = returnedActualState{fakeMAL: remote, wrongProgress: true}
	history, notifications := captureOutcomes(s)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	require.NoError(t, s.execute(ctx, j))
	d, err := s.Detail(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "needs_review", d.Event.Status)
	require.NotNil(t, d.Attempts[0].Observed)
	require.Equal(t, 1, d.Attempts[0].Observed.EpisodesWatched)
	require.Equal(t, 4, d.Attempts[0].Preview.After.EpisodesWatched)
	require.Equal(t, 1, (*history)[0].ListStatus.NumEpisodesWatched)
	require.Equal(t, []bool{false}, *notifications)
}
func TestNoChangeAndRecoveryDoNotAnnounceAnotherSuccess(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		s, _, remote, _, ctx := fixture(t, true)
		seed(t, s, direct(1, 4))
		remote.state.EpisodesWatched = 8
		history, notifications := captureOutcomes(s)
		p, err := s.Preview(ctx, 1, nil)
		require.NoError(t, err)
		j, err := s.Replay(ctx, 1, p.ID)
		require.NoError(t, err)
		require.NoError(t, s.execute(ctx, j))
		require.Zero(t, remote.writes)
		require.Empty(t, *notifications)
		require.Len(t, *history, 1)
		d, err := s.Detail(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "no_change_read", d.Attempts[0].ObservationSource)
		require.Equal(t, 8, d.Attempts[0].Observed.EpisodesWatched)
	})
	t.Run("uncertain then reconciled", func(t *testing.T) {
		s, db, remote, _, ctx := fixture(t, true)
		seed(t, s, direct(1, 4))
		remote.failWrite = true
		remote.applyOnFailure = true
		history, notifications := captureOutcomes(s)
		p, err := s.Preview(ctx, 1, nil)
		require.NoError(t, err)
		j, err := s.Replay(ctx, 1, p.ID)
		require.NoError(t, err)
		require.NoError(t, s.execute(ctx, j))
		require.Equal(t, []bool{false}, *notifications)
		require.True(t, s.reconcile(ctx, j, p))
		require.True(t, s.reconcile(ctx, j, p))
		require.Equal(t, []bool{false}, *notifications)
		require.Equal(t, 1, remote.writes)
		require.Len(t, *history, 2)
		d, err := s.Detail(ctx, 1)
		require.NoError(t, err)
		require.Len(t, d.Attempts, 2)
		require.Nil(t, d.Attempts[0].Observed)
		require.Equal(t, "reconciliation", d.Attempts[1].ObservationSource)
		require.Equal(t, 4, d.Attempts[1].Observed.EpisodesWatched)
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_attempt`).Scan(&count))
		require.Equal(t, 2, count)
	})
}
func TestCurrentPlexUserAndLibraryEligibilityIsRechecked(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	settings := &domain.PlexSettings{PlexUser: "Plex user", AnimeLibraries: []string{"Anime"}}
	checker := PlexEligibility{Settings: func(context.Context) (*domain.PlexSettings, error) { return settings, nil }, Identity: func(context.Context, *domain.PlexSettings) (string, error) {
		t.Fatal("disabled Plex client must not contact server")
		return "", nil
	}}
	s.SetEligibility(checker.Check)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready)
	require.NotEmpty(t, p.Warnings)
	settings.PlexUser = "Different user"
	blocked, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.False(t, blocked.Ready)
	require.Equal(t, "EVENT_NOT_ELIGIBLE", blocked.Code)
	_, err = s.Replay(ctx, 1, p.ID)
	require.Equal(t, "EVENT_NOT_ELIGIBLE", err.(*Error).Code)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_job`).Scan(&count))
	require.Zero(t, count)
	settings.PlexUser = "Plex user"
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	settings.AnimeLibraries = []string{"Different library"}
	require.NoError(t, s.execute(ctx, j))
	job, err := s.Job(ctx, j.ID)
	require.NoError(t, err)
	require.Equal(t, "EVENT_NOT_ELIGIBLE", job.Code)
	require.Equal(t, "needs_review", job.Status)
	require.Zero(t, remote.writes)
	saved, _, err := s.input(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Plex user", saved.Account)
	require.Equal(t, "Anime", saved.Library)
}
func TestEligibilityCheckedImmediatelyBeforeSendAndOnLiveIntake(t *testing.T) {
	s, db, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	j, err := s.Replay(ctx, 1, p.ID)
	require.NoError(t, err)
	checks := 0
	s.SetEligibility(func(context.Context, Input) ([]string, error) {
		checks++
		if checks >= 3 {
			return nil, fail("EVENT_NOT_ELIGIBLE", "Library disabled during planning.")
		}
		return nil, nil
	})
	require.NoError(t, s.execute(ctx, j))
	require.Equal(t, 3, checks)
	require.Zero(t, remote.writes)
	job, err := s.Job(ctx, j.ID)
	require.NoError(t, err)
	require.Equal(t, "EVENT_NOT_ELIGIBLE", job.Code)
	seed(t, s, direct(2, 5))
	a := &domain.AnimeUpdate{PlexId: 2, SourceDB: domain.MAL, SourceId: 384, SeasonNum: 1, EpisodeNum: 5}
	require.Error(t, s.Enqueue(ctx, a, domain.PlexScrobbleEvent))
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_job WHERE event_id=2`).Scan(&count))
	require.Zero(t, count)
}
func TestConfiguredServerEligibilityAndUnavailableSettings(t *testing.T) {
	s, _, remote, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	settings := &domain.PlexSettings{PlexUser: "Plex user", AnimeLibraries: []string{"Anime"}, PlexClientEnabled: true, Host: "plex.test"}
	server := "server-B"
	calls := 0
	checker := PlexEligibility{Settings: func(context.Context) (*domain.PlexSettings, error) { return settings, nil }, Identity: func(context.Context, *domain.PlexSettings) (string, error) { calls++; return server, nil }}
	s.SetEligibility(checker.Check)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.False(t, p.Ready)
	require.Equal(t, "EVENT_NOT_ELIGIBLE", p.Code)
	require.Equal(t, 1, calls)
	server = "server-A"
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready, p.Message)
	settings.PlexClientEnabled = false
	server = "other-server"
	beforeCalls := calls
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.True(t, p.Ready)
	require.Equal(t, beforeCalls, calls)
	require.NotEmpty(t, p.Warnings)
	checker.Settings = func(context.Context) (*domain.PlexSettings, error) {
		return nil, errors.New("private setup must not leak")
	}
	s.SetEligibility(checker.Check)
	p, err = s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.Equal(t, "ELIGIBILITY_UNAVAILABLE", p.Code)
	require.NotContains(t, p.Message, "private setup")
	require.Zero(t, remote.writes)
}
func TestMissingEligibilityCheckerFailsClosed(t *testing.T) {
	s, _, _, _, ctx := fixture(t, true)
	seed(t, s, direct(1, 4))
	s.SetEligibility(nil)
	p, err := s.Preview(ctx, 1, nil)
	require.NoError(t, err)
	require.False(t, p.Ready)
	require.Equal(t, "ELIGIBILITY_UNAVAILABLE", p.Code)
}
func TestUnknownHistoricalServerIsNotGuessed(t *testing.T) {
	c := PlexEligibility{Settings: func(context.Context) (*domain.PlexSettings, error) {
		return &domain.PlexSettings{PlexUser: "viewer", AnimeLibraries: []string{"Anime"}, PlexClientEnabled: true, Host: "plex.test"}, nil
	}, Identity: func(context.Context, *domain.PlexSettings) (string, error) {
		t.Fatal("unknown original server must not be replaced by current server")
		return "", nil
	}}
	warnings, err := c.Check(context.Background(), Input{Account: "viewer", Library: "Anime"})
	require.NoError(t, err)
	require.NotEmpty(t, warnings)
}

func TestDetailPlexPreparationFailureDoesNotInventMALAttempt(t *testing.T) {
	for _, code := range []string{"AGENT_NOT_SUPPORTED", "EXTRACTION_FAILED", "UNKNOWN_ERROR"} {
		t.Run(code, func(t *testing.T) {
			s, db, remote, _, ctx := fixture(t, false)
			seed(t, s, direct(1, 2))
			_, err := db.Exec(`UPDATE plex_payload SET success=0,error_type=?,error_msg='private-response-must-not-leak' WHERE id=1`, code)
			require.NoError(t, err)
			detail, err := s.Detail(ctx, 1)
			require.NoError(t, err)
			require.Equal(t, "failed", detail.Event.Status)
			require.Equal(t, code, detail.Event.ErrorCode)
			require.Zero(t, detail.Event.MALID)
			require.NotNil(t, detail.Original)
			require.Equal(t, "failed", detail.Original.Status)
			require.Len(t, detail.Attempts, 1)
			require.Equal(t, "extraction", detail.Attempts[0].Kind)
			require.Nil(t, detail.Attempts[0].Preview)
			require.Nil(t, detail.Attempts[0].Observed)
			require.Contains(t, detail.Event.ErrorMessage, "MAL sync was attempted")
			require.NotContains(t, marshal(detail), "private-response-must-not-leak")
			require.Zero(t, remote.writes)
		})
	}
}
