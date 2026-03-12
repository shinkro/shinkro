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
	log                zerolog.Logger
	repo               domain.AnimeUpdateRepo
	animeService       anime.Service
	mapService         mapping.Service
	malauthService     malauth.Service
	anilistAuthService anilistauth.Service
	bus                EventBus.Bus
}

func NewService(log zerolog.Logger, repo domain.AnimeUpdateRepo, animeSvc anime.Service, mapSvc mapping.Service, malauthSvc malauth.Service, anilistAuthSvc anilistauth.Service, bus EventBus.Bus) Service {
	return &service{
		log:                log.With().Str("module", "animeUpdate").Logger(),
		repo:               repo,
		animeService:       animeSvc,
		mapService:         mapSvc,
		malauthService:     malauthSvc,
		anilistAuthService: anilistAuthSvc,
		bus:                bus,
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
	return err
}

func (s *service) handleEvent(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) error {
	if anime.SourceDB == domain.MAL {
		anime.MALId = anime.SourceId
		return s.updateAndStore(ctx, anime, isScrobble)
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

	if anime.SeasonNum == 1 {
		return s.updateFromDBAndStore(ctx, anime, isScrobble)
	}

	s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMappingNotFound, err.Error())
	return err
}

func (s *service) updateAndStore(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) error {
	client, err := s.malauthService.GetMalClient(ctx)
	if err != nil {
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAuthFailed, err.Error())
		return err
	}

	if err := s.fetchAnimeDetails(ctx, client, anime); err != nil {
		s.publishAnimeUpdateFailed(anime, domain.AnimeUpdateErrorMALAPIFetchFailed, err.Error())
		return err
	}

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

	// Sync to AniList if configured (non-blocking, errors are logged only)
	anilistID, anilistSynced := s.syncToAniList(ctx, anime, isScrobble)

	anime.Status = domain.AnimeUpdateStatusSuccess

	if err := s.Store(ctx, anime); err != nil {
		return err
	}

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

// syncToAniList syncs the anime update to AniList if configured.
// Returns the AniList ID and whether the sync was successful.
// Errors are logged but never propagate — MAL sync is the primary operation.
func (s *service) syncToAniList(ctx context.Context, anime *domain.AnimeUpdate, isScrobble bool) (int, bool) {
	if s.anilistAuthService == nil {
		return 0, false
	}

	anilistID, err := s.resolveAniListID(ctx, anime.MALId)
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
		params := s.buildAnilistParams(anilistID, anime)
		if err := s.anilistAuthService.UpdateAnimeEntry(ctx, params); err != nil {
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
		if err := s.anilistAuthService.UpdateAnimeScore(ctx, anilistID, rating); err != nil {
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

// buildAnilistParams translates the shinkro AnimeUpdate into AniList params,
// replicating the same MAL logic: start date on ep1, finish date on completion, rewatch tracking.
func (s *service) buildAnilistParams(anilistID int, anime *domain.AnimeUpdate) anilistauth.AnilistUpdateParams {
	details := anime.ListDetails
	ep := anime.EpisodeNum

	params := anilistauth.AnilistUpdateParams{
		AnilistID: anilistID,
		Progress:  ep,
	}

	isCompleted := details.TotalEpisodeNum > 0 && ep >= details.TotalEpisodeNum
	isFirstEp := ep == 1 && details.WatchedNum == 0

	switch {
	case details.Status == "completed" && !isCompleted:
		// Already completed before — now rewatching
		params.Status = anilistauth.AnilistStatusRepeating
		params.Repeat = details.RewatchNum
	case isCompleted:
		params.Status = anilistauth.AnilistStatusCompleted
		now := time.Now()
		params.CompletedAt = &now
		if isFirstEp {
			params.StartedAt = &now
		}
	default:
		params.Status = anilistauth.AnilistStatusCurrent
		if isFirstEp {
			now := time.Now()
			params.StartedAt = &now
		}
	}

	return params
}

// resolveAniListID queries AniList GraphQL to find the AniList ID from a MAL ID.
func (s *service) resolveAniListID(ctx context.Context, malID int) (int, error) {
	accessToken, err := s.anilistAuthService.GetAccessToken(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "anilist not configured or token invalid")
	}

	query := `query ($malId: Int) { Media(idMal: $malId, type: ANIME) { id } }`
	variables := map[string]interface{}{"malId": malID}

	type mediaResponse struct {
		Data struct {
			Media struct {
				ID int `json:"id"`
			} `json:"Media"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	var result mediaResponse
	if err := s.anilistGraphQLQuery(ctx, accessToken, query, variables, &result); err != nil {
		return 0, err
	}
	if len(result.Errors) > 0 {
		return 0, errors.Errorf("anilist graphql error: %s", result.Errors[0].Message)
	}
	if result.Data.Media.ID == 0 {
		return 0, errors.Errorf("anilist: no media found for MAL ID %d", malID)
	}
	return result.Data.Media.ID, nil
}

func (s *service) anilistGraphQLQuery(ctx context.Context, accessToken, query string, variables map[string]interface{}, result interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return errors.Wrap(err, "failed to marshal graphql request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, domain.AnilistGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response")
	}
	return json.Unmarshal(respBody, result)
}

// publishAnimeUpdateFailed publishes failure event and stores failed anime_update record
func (s *service) publishAnimeUpdateFailed(anime *domain.AnimeUpdate, errorType domain.AnimeUpdateErrorType, errorMessage string) {
	anime.Status = domain.AnimeUpdateStatusFailed
	anime.ErrorType = errorType
	anime.ErrorMessage = errorMessage

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

func (s *service) fetchAnimeDetails(ctx context.Context, client *mal.Client, anime *domain.AnimeUpdate) error {
	aa, _, err := client.Anime.Details(ctx, anime.MALId, mal.Fields{"num_episodes", "title", "main_picture{medium,large}", "my_list_status{status,num_times_rewatched,num_episodes_watched}"})
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
	)
	anime.UpdateListDetails(details)
	return nil
}

func (s *service) updateRating(ctx context.Context, client *mal.Client, anime *domain.AnimeUpdate) error {
	l, _, err := client.Anime.UpdateMyListStatus(ctx, anime.MALId, mal.Score(anime.Plex.Rating))
	if err != nil {
		return err
	}
	anime.UpdateRatingWithStatus(*l)
	return nil
}

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
