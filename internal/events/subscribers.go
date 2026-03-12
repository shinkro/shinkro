package events

import (
	"context"
	"fmt"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/animeupdate"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/notification"
	"github.com/shinkro/shinkro/internal/plex"
)

type Subscriber struct {
	log      zerolog.Logger
	eventbus EventBus.Bus

	notificationService notification.Service
	plexService         plex.Service
	animeUpdateService  animeupdate.Service
}

func NewSubscribers(log zerolog.Logger, eventbus EventBus.Bus, notificationSvc notification.Service, plexSvc plex.Service, animeUpdateSvc animeupdate.Service) *Subscriber {
	s := &Subscriber{
		log:                 log.With().Str("module", "events").Logger(),
		eventbus:            eventbus,
		notificationService: notificationSvc,
		plexService:         plexSvc,
		animeUpdateService:  animeUpdateSvc,
	}

	s.Register()

	return s
}

func (s *Subscriber) Register() {
	s.eventbus.Subscribe(domain.EventPlexProcessedSuccess, s.handlePlexProcessedSuccess)
	s.eventbus.Subscribe(domain.EventPlexProcessedError, s.handlePlexProcessedError)
	s.eventbus.Subscribe(domain.EventNotificationSend, s.handleNotificationSend)
	s.eventbus.Subscribe(domain.EventAnimeUpdateSuccess, s.handleAnimeUpdateSuccess)
	s.eventbus.Subscribe(domain.EventAnimeUpdateFailed, s.handleAnimeUpdateFailed)
}

func (s *Subscriber) handlePlexProcessedSuccess(event *domain.PlexProcessedSuccessEvent) {
	s.log.Trace().
		Str("event", domain.EventPlexProcessedSuccess).
		Int64("plexID", event.PlexID).
		Msg("plex processed successfully (extraction complete)")

	success := true
	if err := s.plexService.UpdateStatus(context.Background(), event.PlexID, &success, "", ""); err != nil {
		s.log.Error().Err(err).Msg("failed to store plex success status")
	}

	// Trigger MAL update asynchronously as a side effect
	if event.AnimeUpdate != nil && event.Plex != nil {
		go func() {
			bgCtx := context.Background()
			if err := s.animeUpdateService.UpdateAnimeList(bgCtx, event.AnimeUpdate, event.Plex.Event); err != nil {
				// Error is already handled and published by UpdateAnimeList
				s.log.Debug().Err(err).Msg("MAL update failed")
			}
		}()
	}
}

func (s *Subscriber) handlePlexProcessedError(event *domain.PlexProcessedErrorEvent) {
	s.log.Trace().
		Str("event", domain.EventPlexProcessedError).
		Int64("plexID", event.PlexID).
		Str("error", event.ErrorMessage).
		Msg("plex processing error")

	// Store error status with error type
	success := false
	if err := s.plexService.UpdateStatus(context.Background(), event.PlexID, &success, event.ErrorType, event.ErrorMessage); err != nil {
		s.log.Error().Err(err).Msg("failed to store plex error status")
	}

	// Send error notification with detailed context
	subject := s.buildPlexErrorSubject(event.ErrorType)
	message := s.buildPlexErrorMessage(event.ErrorType, event.ErrorMessage, event.Plex)

	payload := domain.NotificationPayload{
		Message:      message,
		Subject:      subject,
		AnimeLibrary: event.Plex.Metadata.LibrarySectionTitle,
		MediaName:    s.getPlexTitle(event.Plex),
		PlexEvent:    event.Plex.Event,
		PlexSource:   event.Plex.Source,
		Timestamp:    event.Timestamp,
	}
	s.notificationService.Send(domain.NotificationEventPlexProcessingError, payload)
}

func (s *Subscriber) handleNotificationSend(event *domain.NotificationSendEvent) {
	s.notificationService.Send(event.Event, event.Payload)
}

func (s *Subscriber) handleAnimeUpdateSuccess(event *domain.AnimeUpdateSuccessEvent) {
	s.log.Trace().
		Str("event", domain.EventAnimeUpdateSuccess).
		Int64("plexID", event.PlexID).
		Int("malID", event.AnimeUpdate.MALId).
		Msg("anime update succeeded")

	// Send success notification with detailed info
	if event.AnimeUpdate != nil {
		// Build the list of services that were actually updated
		updatedServices := []string{"MyAnimeList"}
		if event.AnilistSynced {
			updatedServices = append(updatedServices, "AniList")
		}

		payload := domain.NotificationPayload{
			MediaName:       event.AnimeUpdate.ListDetails.Title,
			MALID:           event.AnimeUpdate.MALId,
			AnilistID:       event.AnilistID,
			UpdatedServices: updatedServices,
			AnimeLibrary:    event.AnimeUpdate.Plex.Metadata.LibrarySectionTitle,
			EpisodesWatched: event.AnimeUpdate.ListStatus.NumEpisodesWatched,
			EpisodesTotal:   event.AnimeUpdate.ListDetails.TotalEpisodeNum,
			TimesRewatched:  event.AnimeUpdate.ListStatus.NumTimesRewatched,
			PictureURL:      event.AnimeUpdate.ListDetails.PictureURL,
			StartDate:       event.AnimeUpdate.ListStatus.StartDate,
			FinishDate:      event.AnimeUpdate.ListStatus.FinishDate,
			AnimeStatus:     string(event.AnimeUpdate.ListStatus.Status),
			Score:           event.AnimeUpdate.ListStatus.Score,
			PlexEvent:       event.AnimeUpdate.Plex.Event,
			PlexSource:      event.AnimeUpdate.Plex.Source,
			Timestamp:       event.Timestamp,
		}
		s.notificationService.Send(domain.NotificationEventSuccess, payload)
	}
}

func (s *Subscriber) handleAnimeUpdateFailed(event *domain.AnimeUpdateFailedEvent) {
	s.log.Trace().
		Str("event", domain.EventAnimeUpdateFailed).
		Int64("plexID", event.AnimeUpdate.Plex.ID).
		Str("errorType", string(event.ErrorType)).
		Str("error", event.ErrorMessage).
		Msg("anime update failed")


	// Extract anime title for better error messages
	animeTitle := s.getAnimeTitle(event.AnimeUpdate)

	// Send detailed error notification
	subject := s.buildErrorSubject(event.ErrorType)
	message := s.buildErrorMessage(event.ErrorType, event.ErrorMessage, event.AnimeUpdate)

	payload := domain.NotificationPayload{
		Message:      message,
		Subject:      subject,
		AnimeLibrary: event.AnimeUpdate.Plex.Metadata.LibrarySectionTitle,
		MediaName:    animeTitle,
		MALID:        event.AnimeUpdate.MALId,
		PlexEvent:    event.AnimeUpdate.Plex.Event,
		PlexSource:   event.AnimeUpdate.Plex.Source,
		Timestamp:    event.Timestamp,
	}
	s.notificationService.Send(domain.NotificationEventAnimeUpdateError, payload)
}

// getAnimeTitle extracts anime title from various sources
func (s *Subscriber) getAnimeTitle(anime *domain.AnimeUpdate) string {
	if anime.ListDetails.Title != "" {
		return anime.ListDetails.Title
	}
	if anime.Plex != nil {
		if anime.Plex.Metadata.Type == "movie" {
			if anime.Plex.Metadata.Title != "" {
				return anime.Plex.Metadata.Title
			}
		} else {
			// For episodes, the show title is in GrandparentTitle
			if anime.Plex.Metadata.GrandparentTitle != "" {
				return anime.Plex.Metadata.GrandparentTitle
			}
		}
	}
	if anime.SourceDB != "" && anime.SourceId > 0 {
		return fmt.Sprintf("%s ID: %d", string(anime.SourceDB), anime.SourceId)
	}
	return "Unknown"
}

// buildErrorSubject creates a user-friendly error subject
func (s *Subscriber) buildErrorSubject(errorType domain.AnimeUpdateErrorType) string {
	switch errorType {
	case domain.AnimeUpdateErrorMALAuthFailed:
		return "MAL Authentication Failed"
	case domain.AnimeUpdateErrorMappingNotFound:
		return "Mapping Not Found"
	case domain.AnimeUpdateErrorAnimeNotInDB:
		return "Anime Not in Database"
	case domain.AnimeUpdateErrorMALAPIFetchFailed:
		return "MAL API Error"
	case domain.AnimeUpdateErrorMALAPIUpdateFailed:
		return "MAL Update Failed"
	default:
		return "Anime Update Failed"
	}
}

// buildErrorMessage creates a detailed error message with actionable information
func (s *Subscriber) buildErrorMessage(errorType domain.AnimeUpdateErrorType, errorMsg string, anime *domain.AnimeUpdate) string {
	baseMsg := fmt.Sprintf("Failed to update MyAnimeList for: %s\n\n", s.getAnimeTitle(anime))

	switch errorType {
	case domain.AnimeUpdateErrorMALAuthFailed:
		return baseMsg + fmt.Sprintf("Error: %s\n\nAction Required: Please re-authenticate with MyAnimeList in settings.", errorMsg)
	case domain.AnimeUpdateErrorMappingNotFound:
		return baseMsg + fmt.Sprintf("Error: %s\n\nSource: %s ID %d (Season %d)\n\nAction Required: Add a mapping for this anime.",
			errorMsg, string(anime.SourceDB), anime.SourceId, anime.SeasonNum)
	case domain.AnimeUpdateErrorAnimeNotInDB:
		return baseMsg + fmt.Sprintf("Error: %s\n\nSource: %s ID %d\n\nAction Required: Add a mapping for this anime.",
			errorMsg, string(anime.SourceDB), anime.SourceId)
	case domain.AnimeUpdateErrorMALAPIFetchFailed:
		return baseMsg + fmt.Sprintf("Error: Failed to fetch anime details from MAL API\n\nDetails: %s\n\nThis might be a temporary MAL API issue.", errorMsg)
	case domain.AnimeUpdateErrorMALAPIUpdateFailed:
		return baseMsg + fmt.Sprintf("Error: Failed to update MAL list\n\nDetails: %s\n\nThis might be a temporary MAL API issue.", errorMsg)
	default:
		return baseMsg + fmt.Sprintf("Error: %s", errorMsg)
	}
}

// getPlexTitle extracts title from Plex metadata
func (s *Subscriber) getPlexTitle(plex *domain.Plex) string {
	if plex.Metadata.Type == "movie" {
		if plex.Metadata.Title != "" {
			return plex.Metadata.Title
		}
		return "Unknown"
	}
	// For episodes, the show title is in GrandparentTitle
	if plex.Metadata.GrandparentTitle != "" {
		return plex.Metadata.GrandparentTitle
	}
	return "Unknown"
}

// buildPlexErrorSubject creates a user-friendly error subject for plex errors
func (s *Subscriber) buildPlexErrorSubject(errorType domain.PlexErrorType) string {
	switch errorType {
	case domain.PlexErrorAgentNotSupported:
		return "Unsupported Metadata Agent"
	case domain.PlexErrorExtractionFailed:
		return "Failed to Extract Anime Info"
	default:
		return "Plex Payload Processing Failed"
	}
}

// buildPlexErrorMessage creates a detailed error message with actionable information
func (s *Subscriber) buildPlexErrorMessage(errorType domain.PlexErrorType, errorMsg string, plex *domain.Plex) string {
	title := s.getPlexTitle(plex)
	baseMsg := fmt.Sprintf("Failed to process Plex payload for: %s\n\n", title)

	switch errorType {
	case domain.PlexErrorAgentNotSupported:
		return baseMsg + fmt.Sprintf("Error: %s\n\nAction Required: Please configure a supported agent (HAMA, MAL Agent, or Plex Agent) in your Plex library settings.", errorMsg)
	case domain.PlexErrorExtractionFailed:
		return baseMsg + fmt.Sprintf("Error: %s\n\nThis indicates an issue with extracting the anime ID from the metadata. Create issue on github.com/shinkro/shinkro", errorMsg)
	default:
		return baseMsg + fmt.Sprintf("Error: %s", errorMsg)
	}
}
