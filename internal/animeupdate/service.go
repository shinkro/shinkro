package animeupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/nstratos/go-myanimelist/mal"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/anime"
	"github.com/shinkro/shinkro/internal/anilistauth"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/malauth"
	"github.com/shinkro/shinkro/internal/mapping"
)

type Service interface {
	Store(ctx context.Context, animeupdate *domain.AnimeUpdate) error
	GetByID(ctx context.Context, req *domain.GetAnimeUpdateRequest) (*domain.AnimeUpdate, error)
	UpdateAnimeList(ctx context.Context, anime *domain.AnimeUpdate, event domain.PlexEvent) error
	Count(ctx context.Context) (int, error)
	GetRecentUnique(ctx context.Context, limit int) ([]*domain.AnimeUpdate, error)
	GetByPlexID(ctx context.Context, plexID int64) (*domain.AnimeUpdate, error)
	GetByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error)
	FindAllWithFilters(ctx context.Context, params domain.AnimeUpdateQueryParams) (*domain.FindAnimeUpdatesResponse, error)
}

type service struct {
	log            zerolog.Logger
	repo           domain.AnimeUpdateRepo
	animeService   anime.Service
	mapService     mapping.Service
	malauthService malauth.Service
	anilistAuthService anilistauth.Service
	bus            EventBus.Bus
}

func NewService(log zerolog.Logger, repo domain.AnimeUpdateRepo, animeSvc anime.Service, mapSvc mapping.Service, malauthSvc malauth.Service, anilistAuthSvc anilistauth.Service, bus EventBus.Bus) Service {
	return &service{
		log:            log.With().Str("module", "animeUpdate").Logger(),
		repo:           repo,
		animeService:   animeSvc,
		mapService:     mapSvc,
		malauthService: malauthSvc,
		anilistAuthService: anilistAuthSvc,
		bus:            bus,
	}
}

func (s *service) Store(ctx context.Context, animeupdate *domain.AnimeUpdate) error {
	if err := s.repo.Store(ctx, animeupdate); err != nil {
		return err
	}

	s.log.Trace().
		Int("malID", animeupdate.MALId).
		Int64("plexID", animeupdate.PlexId).
		Msg("anime update stored")

	return nil
}

func (s *service) GetByID(ctx context.Context, req *domain.GetAnimeUpdateRequest) (*domain.AnimeUpdate, error) {
	return s.repo.GetByID(ctx, req)
}

func (s *service) UpdateAnimeList(ctx context.Context, anime *domain.AnimeUpdate, event domain.PlexEvent) error {
	var err error
	switch event {
	case domain.PlexRateEvent:
		err = s.handleEvent(ctx, anime, false)
	case domain.PlexScrobbleEvent:
		err = s.handleEvent(ctx, anime, true)
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *service) handleEvent(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) error {
	if anime.SourceDB == domain.MAL {
		anime.MALId = anime.SourceId
		return s.updateAndStore(ctx, anime, isScrobble)
	}

	// For season 1, the AniDB ID identifies the series, so a direct DB lookup is exact
	// and avoids TVDB conversion collapsing multiple series onto a shared TVDB ID.
	// For season > 1 the season number is the differentiator, so the community map
	// (keyed by TVDB ID + season) must be used instead.
	if anime.SourceDB == domain.AniDB && anime.SeasonNum == 1 {
		req := &domain.GetAnimeRequest{IDtype: anime.SourceDB, Id: anime.SourceId}
		if animeFromDB, err := s.animeService.GetByID(ctx, req); err == nil && animeFromDB.MALId > 0 {
			anime.MALId = animeFromDB.MALId
			return s.updateAndStore(ctx, anime, isScrobble)
		}
		// Not in DB — fall through to TVDB conversion + community map
	}

	convertedAnime := s.convertAniDBToTVDB(ctx, anime)
	animeMap, err := s.mapService.CheckForAnimeinMap(ctx, convertedAnime)
	if err == nil {
		anime.MALId = animeMap.Malid
		if isScrobble {
			anime.EpisodeNum = animeMap.CalculateEpNum(anime.EpisodeNum)
		}
		return s.updateAndStore(ctx, anime, isScrobble)
	}

	// Mapping not found - try database lookup for season 1
	if anime.SeasonNum == 1 {
		return s.updateFromDBAndStore(ctx, anime, isScrobble)
	}

	// Mapping not found and not season 1 - publish error
	s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMappingNotFound, err.Error())
	return err
}

func (s *service) updateAndStore(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) error {
	client, err := s.malauthService.GetMalClient(ctx)
	if err != nil {
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAuthFailed, err.Error())
		return err
	}

	// Fetch current anime list details from MAL API
	if err := s.fetchAnimeDetails(ctx, client, anime); err != nil {
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAPIFetchFailed, err.Error())
		return err
	}

	// Update MAL based on event type
	if isScrobble {
		if err := s.updateWatchStatus(ctx, client, anime); err != nil {
			s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAPIUpdateFailed, err.Error())
			return err
		}
	} else {
		if err := s.updateRating(ctx, client, anime); err != nil {
			s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAPIUpdateFailed, err.Error())
			return err
		}
	}

	s.log.Info().Interface("status", anime.ListStatus).Msg("MyAnimeList Updated Successfully")

	// Set status to SUCCESS before storing
	anime.Status = domain.AnimeUpdateStatusSuccess
	
	// Store the update
	if err := s.Store(ctx, anime); err != nil {
		return err
	}

	// Sync to AniList if configured (non-blocking — errors are logged only)
	anilistID, anilistSynced := s.syncToAniList(ctx, anime, isScrobble)

	// Publish success event
	s.bus.Publish(domain.EventAnimeUpdateSuccess, &domain.AnimeUpdateSuccessEvent{
		PlexID:        anime.PlexId,
		AnimeUpdate:   anime,
		AnilistSynced: anilistSynced,
		AnilistID:     anilistID,
		Timestamp:     time.Now(),
	})

	return nil
}

func (s *service) updateFromDBAndStore(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) error {
	req := &domain.GetAnimeRequest{
		IDtype: anime.SourceDB,
		Id:     anime.SourceId,
	}

	animeFromDB, err := s.animeService.GetByID(ctx, req)
	if err != nil {
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorAnimeNotInDB, err.Error())
		return err
	}

	s.log.Debug().Int("malId", animeFromDB.MALId).Msg("Anime from DB")
	if animeFromDB.MALId == 0 {
		errMsg := "could not retrieve malid from internal database"
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorAnimeNotInDB, errMsg)
		return errors.New(errMsg)
	}

	anime.MALId = animeFromDB.MALId
	return s.updateAndStore(ctx, anime, isScrobble)
}

// publishAnimeUpdateFailed publishes failure event and stores failed anime_update record
func (s *service) publishAnimeUpdateFailed(anime *domain.AnimeUpdate, errorType domain.AnimeUpdateErrorType, errorMessage string) {
	// Set status fields for failed update
	anime.Status = domain.AnimeUpdateStatusFailed
	anime.ErrorType = errorType
	anime.ErrorMessage = errorMessage
	
	// Store the failed update record
	if err := s.Store(context.Background(), anime); err != nil {
		s.log.Error().Err(err).Msg("failed to store anime update failure record")
	}
	
	s.bus.Publish(domain.EventAnimeUpdateFailed, &domain.AnimeUpdateFailedEvent{
		AnimeUpdate:  anime,
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		Timestamp:    time.Now(),
	})
}

// fetchAnimeDetails calls MAL API to get current anime list details
func (s *service) fetchAnimeDetails(ctx context.Context, client *mal.Client, anime *domain.AnimeUpdate) error {
	aa, _, err := client.Anime.Details(ctx, anime.MALId, mal.Fields{"num_episodes", "title", "main_picture{medium,large}", "my_list_status{status,num_times_rewatched,num_episodes_watched,start_date,finish_date}"})
	if err != nil {
		return err
	}

	details := domain.BuildListDetailsFromMALResponse(
		aa.MyListStatus.Status,
		aa.MyListStatus.NumTimesRewatched,
		aa.NumEpisodes,
		aa.MyListStatus.NumEpisodesWatched,
		aa.Title,
		aa.MainPicture.Medium,
		aa.MyListStatus.StartDate,
		aa.MyListStatus.FinishDate,
	)
	anime.UpdateListDetails(details)

	return nil
}

// updateRating calls MAL API to update rating and updates domain with result
func (s *service) updateRating(ctx context.Context, client *mal.Client, anime *domain.AnimeUpdate) error {
	l, _, err := client.Anime.UpdateMyListStatus(ctx, anime.MALId, mal.Score(anime.Plex.Rating))
	if err != nil {
		return err
	}

	anime.UpdateRatingWithStatus(*l)
	return nil
}

// updateWatchStatus calls MAL API to update watch status and updates domain with result
func (s *service) updateWatchStatus(ctx context.Context, client *mal.Client, anime *domain.AnimeUpdate) error {
	options, err := anime.BuildWatchStatusOptions()
	if err != nil {
		return err
	}

	l, _, err := client.Anime.UpdateMyListStatus(ctx, anime.MALId, options...)
	if err != nil {
		return err
	}

	anime.UpdateWatchStatusWithStatus(*l)
	return nil
}

func (s *service) convertAniDBToTVDB(ctx context.Context, anime *domain.AnimeUpdate) *domain.AnimeUpdate {
	if anime.SourceDB != domain.AniDB {
		return anime
	}

	req := &domain.GetAnimeRequest{
		IDtype: anime.SourceDB,
		Id:     anime.SourceId,
	}

	aa, err := s.animeService.GetByID(ctx, req)
	if err != nil {
		return anime
	}

	newAnime := *anime
	if aa.TVDBId > 0 {
		newAnime.SourceDB = domain.TVDB
		newAnime.SourceId = aa.TVDBId
		s.log.Debug().Int("converted tvdbId", aa.TVDBId).Msg("Converted Anime to TVDB")
	} else {
		return anime
	}

	return &newAnime
}

func (s *service) Count(ctx context.Context) (int, error) {
	return s.repo.Count(ctx)
}

func (s *service) GetRecentUnique(ctx context.Context, limit int) ([]*domain.AnimeUpdate, error) {
	return s.repo.GetRecentUnique(ctx, limit)
}

func (s *service) GetByPlexID(ctx context.Context, plexID int64) (*domain.AnimeUpdate, error) {
	return s.repo.GetByPlexID(ctx, plexID)
}

func (s *service) GetByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error) {
	if len(plexIDs) == 0 {
		return []*domain.AnimeUpdate{}, nil
	}
	return s.repo.GetByPlexIDs(ctx, plexIDs)
}

func (s *service) FindAllWithFilters(ctx context.Context, params domain.AnimeUpdateQueryParams) (*domain.FindAnimeUpdatesResponse, error) {
	return s.repo.FindAllWithFilters(ctx, params)
}

// syncToAniList syncs the anime update to AniList if the service is configured.
// Returns the AniList ID and whether the sync succeeded.
// Errors are always non-fatal — MAL sync is the primary operation.
func (s *service) syncToAniList(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) (int, bool) {
	if s.anilistAuthService == nil {
		return 0, false
	}

	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	anilistID, err := s.resolveAniListID(syncCtx, anime.MALId)
	if err != nil {
		s.log.Debug().Err(err).Int("malID", anime.MALId).Msg("AniList: could not resolve AniList ID, skipping sync")
		s.bus.Publish(domain.EventAnilistSyncFailed, &domain.AnilistSyncFailedEvent{
			AnimeUpdate:  anime,
			AnilistID:    0,
			ErrorMessage: err.Error(),
			Timestamp:    time.Now(),
		})
		return 0, false
	}

	if isScrobble {
		anilistDates, err := s.anilistAuthService.GetAnimeEntry(syncCtx, anilistID)
		if err != nil {
			s.log.Debug().Err(err).Int("anilistID", anilistID).Msg("AniList: could not fetch existing entry dates, proceeding without them")
			anilistDates = &anilistauth.AnilistEntryDates{}
		}
		params := s.buildAnilistParams(anilistID, anime, anilistDates)
		if err := s.anilistAuthService.UpdateAnimeEntry(syncCtx, params); err != nil {
			s.log.Warn().Err(err).Int("anilistID", anilistID).Msg("AniList: failed to update entry")
			s.bus.Publish(domain.EventAnilistSyncFailed, &domain.AnilistSyncFailedEvent{
				AnimeUpdate:  anime,
				AnilistID:    anilistID,
				ErrorMessage: err.Error(),
				Timestamp:    time.Now(),
			})
			return anilistID, false
		}
		s.log.Info().
			Int("anilistID", anilistID).
			Int("episode", anime.EpisodeNum).
			Str("status", string(params.Status)).
			Msg("AniList: entry updated successfully")
	} else {
		rating := 0.0
		if anime.Plex != nil {
			rating = float64(anime.Plex.Rating)
		}
		if err := s.anilistAuthService.UpdateAnimeScore(syncCtx, anilistID, rating); err != nil {
			s.log.Warn().Err(err).Int("anilistID", anilistID).Msg("AniList: failed to update score")
			s.bus.Publish(domain.EventAnilistSyncFailed, &domain.AnilistSyncFailedEvent{
				AnimeUpdate:  anime,
				AnilistID:    anilistID,
				ErrorMessage: err.Error(),
				Timestamp:    time.Now(),
			})
			return anilistID, false
		}
		s.log.Info().Int("anilistID", anilistID).Float64("score", rating).Msg("AniList: score updated successfully")
	}
	return anilistID, true
}

// buildAnilistParams translates a shinkro AnimeUpdate into AniList mutation params.
//
// Date resolution uses a cross-service fallback strategy:
//   - StartedAt:   MAL date if set → keep AniList date if set → set now() only on ep1
//   - CompletedAt: MAL date if set → keep AniList date if set → set now() on completion
//
// This ensures that adding AniList after MAL (or vice versa) will always backfill
// missing dates from whichever service already has them, without overwriting dates
// that are present on both sides.
func (s *service) buildAnilistParams(anilistID int, anime *domain.AnimeUpdate, anilistDates *anilistauth.AnilistEntryDates) anilistauth.AnilistUpdateParams {
	details := anime.ListDetails
	ep := anime.EpisodeNum

	params := anilistauth.AnilistUpdateParams{
		AnilistID: anilistID,
		Progress:  ep,
	}

	isCompleted := details.TotalEpisodeNum > 0 && ep >= details.TotalEpisodeNum
	isFirstEp := ep == 1 && details.WatchedNum == 0

	// Resolve StartedAt:
	// 1) MAL has a date → use it (backfills AniList if missing)
	// 2) AniList already has a date → pass nil (leave it untouched)
	// 3) Neither has a date and it's ep1 → set now()
	// 4) Neither has a date and ep > 1 → pass nil (no date to set)
	var startedAt *time.Time
	if details.MALStartDate != "" {
		if t, err := time.Parse("2006-01-02", details.MALStartDate); err == nil {
			startedAt = &t
		}
	} else if anilistDates.StartedAt == nil && isFirstEp {
		now := time.Now()
		startedAt = &now
	}

	// Resolve CompletedAt (same cross-fallback logic)
	var completedAt *time.Time
	if details.MALFinishDate != "" {
		if t, err := time.Parse("2006-01-02", details.MALFinishDate); err == nil {
			completedAt = &t
		}
	} else if anilistDates.CompletedAt == nil && isCompleted {
		now := time.Now()
		completedAt = &now
	}

	switch {
	case details.Status == "completed" && !isCompleted:
		// Already completed before — now rewatching
		params.Status = anilistauth.AnilistStatusRepeating
		params.Repeat = details.RewatchNum
	case isCompleted:
		params.Status = anilistauth.AnilistStatusCompleted
		params.CompletedAt = completedAt
		params.StartedAt = startedAt
	default:
		params.Status = anilistauth.AnilistStatusCurrent
		params.StartedAt = startedAt
	}

	return params
}

// resolveAniListID queries the AniList GraphQL API to find the AniList ID for a given MAL ID.
func (s *service) resolveAniListID(ctx context.Context, malID int) (int, error) {
	accessToken, err := s.anilistAuthService.GetAccessToken(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "anilist not configured or token invalid")
	}

	query := `query ($malId: Int) { Media(idMal: $malId, type: ANIME) { id } }`
	variables := map[string]interface{}{"malId": malID}

	body, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return 0, errors.Wrap(err, "failed to marshal graphql request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, domain.AnilistGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return 0, errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read response")
	}

	var result struct {
		Data struct {
			Media struct {
				ID int `json:"id"`
			} `json:"Media"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, errors.Wrap(err, "failed to parse response")
	}
	if len(result.Errors) > 0 {
		return 0, errors.Errorf("anilist graphql error: %s", result.Errors[0].Message)
	}
	if result.Data.Media.ID == 0 {
		return 0, errors.Errorf("anilist: no media found for MAL ID %d", malID)
	}
	return result.Data.Media.ID, nil
}

