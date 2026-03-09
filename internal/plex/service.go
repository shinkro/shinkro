package plex

import (
	"context"
	"strconv"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/anime"
	"github.com/shinkro/shinkro/internal/animeupdate"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/malauth"
	"github.com/shinkro/shinkro/internal/mapping"
	"github.com/shinkro/shinkro/internal/plexsettings"
)

type Service interface {
	Store(ctx context.Context, plex *domain.Plex) error
	Get(ctx context.Context, req *domain.GetPlexRequest) (*domain.Plex, error)
	ProcessPlex(ctx context.Context, plex *domain.Plex) error
	GetPlexSettings(ctx context.Context) (*domain.PlexSettings, error)
	CheckPlex(ctx context.Context, plex *domain.Plex, ps *domain.PlexSettings) error
	CountScrobbleEvents(ctx context.Context) (int, error)
	CountRateEvents(ctx context.Context) (int, error)
	GetPlexHistory(ctx context.Context, limit int) ([]domain.PlexHistoryItem, error)
	FindAllWithFilters(ctx context.Context, params domain.PlexPayloadQueryParams) (*domain.FindPlexPayloadsResponse, error)
	Delete(ctx context.Context, req *domain.DeletePlexRequest) error
	UpdateStatus(ctx context.Context, plexID int64, success *bool, errorType domain.PlexErrorType, errorMsg string) error
}

type service struct {
	log                zerolog.Logger
	repo               domain.PlexRepo
	plexettingsService plexsettings.Service
	animeService       anime.Service
	mapService         mapping.Service
	malauthService     malauth.Service
	animeUpdateService animeupdate.Service
	bus                EventBus.Bus
}

func NewService(log zerolog.Logger, plexsettingsSvc plexsettings.Service, repo domain.PlexRepo, animeSvc anime.Service, mapSvc mapping.Service, malauthSvc malauth.Service, animeUpdateSvc animeupdate.Service, bus EventBus.Bus) Service {
	return &service{
		log:                log.With().Str("module", "plex").Logger(),
		repo:               repo,
		plexettingsService: plexsettingsSvc,
		animeService:       animeSvc,
		mapService:         mapSvc,
		malauthService:     malauthSvc,
		animeUpdateService: animeUpdateSvc,
		bus:                bus,
	}
}

func (s *service) Get(ctx context.Context, req *domain.GetPlexRequest) (*domain.Plex, error) {
	return s.repo.Get(ctx, req)
}

func (s *service) Store(ctx context.Context, plex *domain.Plex) error {
	return s.repo.Store(ctx, plex)
}

func (s *service) GetPlexSettings(ctx context.Context) (*domain.PlexSettings, error) {
	return s.plexettingsService.Get(ctx)
}

// CheckPlex validates a Plex payload (user, event, library, media type, rating).
func (s *service) CheckPlex(ctx context.Context, plex *domain.Plex, ps *domain.PlexSettings) error {
	if !plex.IsPlexUserAllowed(ps) {
		return errors.Wrap(errors.New("unauthorized plex user"), plex.Account.Title)
	}

	if !plex.IsEventAllowed() {
		return errors.Wrap(errors.New("plex event not supported"), string(plex.Event))
	}

	if !plex.IsAnimeLibrary(ps) {
		return errors.Wrap(errors.New("plex library not set as an anime library"), plex.Metadata.LibrarySectionTitle)
	}

	if !plex.IsMediaTypeAllowed() {
		return errors.Wrap(errors.New("plex media type not supported"), string(plex.Metadata.Type))
	}

	if !plex.IsRatingAllowed() {
		return errors.Wrap(errors.New("rating was unset, skipped"), strconv.FormatFloat(float64(plex.Rating), 'f', -1, 64))
	}

	return nil
}

func (s *service) ProcessPlex(ctx context.Context, plex *domain.Plex) error {
	// Check if metadata agent is supported
	allowed, agent := plex.IsMetadataAgentAllowed()
	if !allowed {
		err := errors.New("metadata agent not supported")
		s.bus.Publish(domain.EventPlexProcessedError, &domain.PlexProcessedErrorEvent{
			PlexID:       plex.ID,
			Plex:         plex,
			ErrorType:    domain.PlexErrorAgentNotSupported,
			ErrorMessage: err.Error(),
			Timestamp:    time.Now(),
		})
		return err
	}

	a, err := s.extractSourceIdForAnime(ctx, plex, &agent)
	if err != nil {
		s.bus.Publish(domain.EventPlexProcessedError, &domain.PlexProcessedErrorEvent{
			PlexID:       plex.ID,
			Plex:         plex,
			ErrorType:    domain.PlexErrorExtractionFailed,
			ErrorMessage: err.Error(),
			Timestamp:    time.Now(),
		})
		return err
	}

	// Publish success event - Plex processing succeeded (metadata extraction worked)
	// Event subscriber will trigger MAL update asynchronously
	s.bus.Publish(domain.EventPlexProcessedSuccess, &domain.PlexProcessedSuccessEvent{
		PlexID:      plex.ID,
		Plex:        plex,
		AnimeUpdate: a,
		Timestamp:   time.Now(),
	})

	return nil
}

func (s *service) extractSourceIdForAnime(ctx context.Context, plex *domain.Plex, agent *domain.PlexSupportedAgents) (*domain.AnimeUpdate, error) {
	source, id, err := s.getSourceIDFromAgent(ctx, plex, agent)
	if err != nil {
		return nil, err
	}

	a := plex.SetAnimeFields(source, id)
	return &a, nil
}

func (s *service) getSourceIDFromAgent(ctx context.Context, p *domain.Plex, agent *domain.PlexSupportedAgents) (domain.PlexSupportedDBs, int, error) {
	switch *agent {
	case domain.HAMA, domain.MALAgent:
		return p.Metadata.GUID.HamaMALAgent(*agent)
	case domain.PlexAgent:
		return s.plexettingsService.HandlePlexAgent(ctx, p)
	}
	return "", 0, errors.New("unknown agent")
}

func (s *service) CountScrobbleEvents(ctx context.Context) (int, error) {
	return s.repo.CountScrobbleEvents(ctx)
}

func (s *service) CountRateEvents(ctx context.Context) (int, error) {
	return s.repo.CountRateEvents(ctx)
}

func (s *service) GetPlexHistory(ctx context.Context, limit int) ([]domain.PlexHistoryItem, error) {
	// Get most recent Plex payloads (now includes status fields)
	plexPayloads, err := s.repo.GetRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Extract Plex IDs for batch fetching
	plexIDs := make([]int64, len(plexPayloads))
	for i, p := range plexPayloads {
		plexIDs[i] = p.ID
	}

	// Get all AnimeUpdates (both success and failure - status is now in anime_update)
	animeUpdates, err := s.getAnimeUpdatesByPlexIDs(ctx, plexIDs)
	if err != nil {
		return nil, err
	}

	// Create map for quick lookup
	animeUpdateMap := make(map[int64]*domain.AnimeUpdate)
	for _, au := range animeUpdates {
		animeUpdateMap[au.PlexId] = au
	}

	// Combine data - status is now in plex and animeUpdate directly
	items := make([]domain.PlexHistoryItem, 0, len(plexPayloads))
	for _, plex := range plexPayloads {
		item := domain.PlexHistoryItem{
			Plex: plex,
		}

		// Add AnimeUpdate if exists (now includes status fields)
		if animeUpdate, exists := animeUpdateMap[plex.ID]; exists {
			item.AnimeUpdate = animeUpdate
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *service) getAnimeUpdatesByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error) {
	if len(plexIDs) == 0 {
		return []*domain.AnimeUpdate{}, nil
	}

	return s.animeUpdateService.GetByPlexIDs(ctx, plexIDs)
}

func (s *service) FindAllWithFilters(ctx context.Context, params domain.PlexPayloadQueryParams) (*domain.FindPlexPayloadsResponse, error) {
	return s.repo.FindAllWithFilters(ctx, params)
}

func (s *service) Delete(ctx context.Context, req *domain.DeletePlexRequest) error {
	return s.repo.Delete(ctx, req)
}

func (s *service) UpdateStatus(ctx context.Context, plexID int64, success *bool, errorType domain.PlexErrorType, errorMsg string) error {
	return s.repo.UpdateStatus(ctx, plexID, success, errorType, errorMsg)
}
