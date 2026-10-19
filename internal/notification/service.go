package notification

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/pkg/errors"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/sync/errgroup"
)

type Service interface {
	Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error)
	Store(ctx context.Context, notification *domain.Notification) error
	Update(ctx context.Context, notification *domain.Notification) error
	Delete(ctx context.Context, id int) error
	Send(event domain.NotificationEvent, payload domain.NotificationPayload)
	Test(ctx context.Context, notification *domain.Notification) error
}

type service struct {
	log     zerolog.Logger
	repo    domain.NotificationRepo
	senders map[int]domain.NotificationSender
}

func NewService(log zerolog.Logger, repo domain.NotificationRepo) Service {
	s := &service{
		log:     log.With().Str("module", "notification").Logger(),
		repo:    repo,
		senders: make(map[int]domain.NotificationSender),
	}

	s.registerSenders()

	return s
}

func (s *service) Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error) {
	notifications, count, err := s.repo.Find(ctx, params)
	if err != nil {
		s.log.Error().Err(err).Msgf("could not find notification with params: %+v", params)
		return nil, 0, err
	}

	return notifications, count, err
}

func (s *service) Store(ctx context.Context, notification *domain.Notification) error {
	err := s.repo.Store(ctx, notification)
	if err != nil {
		s.log.Error().Err(err).Msgf("could not store notification: %+v", notification)
		return err
	}

	// register sender
	s.registerSender(notification)

	return nil
}

func (s *service) Update(ctx context.Context, notification *domain.Notification) error {
	err := s.repo.Update(ctx, notification)
	if err != nil {
		s.log.Error().Err(err).Msgf("could not update notification: %+v", notification)
		return err
	}

	// register sender
	s.registerSender(notification)

	return nil
}

func (s *service) Delete(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		s.log.Error().Err(err).Msgf("could not delete notification: %v", id)
		return err
	}

	// delete sender
	delete(s.senders, id)

	return nil
}

func (s *service) registerSenders() {
	notificationSenders, err := s.repo.List(context.Background())
	if err != nil {
		s.log.Error().Err(err).Msg("could not find notifications")
		return
	}

	for _, notificationSender := range notificationSenders {
		s.registerSender(&notificationSender)
	}

	return
}

// registerSender registers an enabled notification via it's id
func (s *service) registerSender(notification *domain.Notification) {
	if !notification.Enabled {
		delete(s.senders, notification.ID)
		return
	}

	switch notification.Type {
	case domain.NotificationTypeDiscord:
		s.senders[notification.ID] = NewDiscordSender(s.log, notification)
	case domain.NotificationTypeGotify:
		s.senders[notification.ID] = NewGotifySender(s.log, notification)
	}

	return
}

// Send notifications
func (s *service) Send(event domain.NotificationEvent, payload domain.NotificationPayload) {
	if len(s.senders) > 0 {
		s.log.Debug().Msgf("sending notification for %v", string(event))
	}

	go func() {
		for _, sender := range s.senders {
			// check if sender is active and have notification types
			if sender.CanSend(event) {
				if err := sender.Send(event, payload); err != nil {
					s.log.Error().Err(err).Msgf("could not send %s notification for %v", sender.Name(), string(event))
				}
			}
		}
	}()

	return
}

func (s *service) Test(ctx context.Context, notification *domain.Notification) error {
	var agent domain.NotificationSender

	// send test events
	events := []domain.NotificationPayload{
		{
			Subject:   "Test Notification",
			Message:   "If you had the strength, you could live. This is our contract. In return for my gift of power, you must grant one of my wishes. If you enter this contract, you will live as a human, but also as one completely different. Different rules, different time, a different life... The power of the king will make you lonely indeed. If you are prepared for that, then...",
			Event:     domain.NotificationEventTest,
			Timestamp: time.Now(),
		},
		{
			Subject:         "SUCCESS",
			Event:           domain.NotificationEventSuccess,
			MediaName:       "Code Geass: Lelouch of the Rebellion",
			MALID:           1575,
			AnilistID:       1575,
			UpdatedServices: []string{"MyAnimeList", "AniList"},
			AnimeLibrary:    "Anime",
			EpisodesWatched: 5,
			EpisodesTotal:   25,
			TimesRewatched:  99,
			StartDate:       "2006-10-06",
			FinishDate:      "2007-07-29",
			AnimeStatus:     "Watching",
			Score:           10,
			PlexEvent:       "media.scrobble",
			PlexSource:      "Plex Webhook",
			Timestamp:       time.Now(),
		},
		{
			Subject:   "New update available!",
			Message:   "v0.2.0",
			Event:     domain.NotificationEventAppUpdateAvailable,
			Timestamp: time.Now(),
		},
		{
			Subject:      "Unsupported Metadata Agent",
			Message:      "Failed to process Plex payload for: Code Geass: Lelouch of the Rebellion\n\nError: metadata agent not supported\n\nAction Required: Please configure a supported agent (HAMA, MAL Agent, or Plex Agent) in your Plex library settings.",
			Event:        domain.NotificationEventPlexProcessingError,
			AnimeLibrary: "Anime",
			MediaName:    "Code Geass: Lelouch of the Rebellion",
			PlexEvent:    "media.scrobble",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
		{
			Subject:      "Failed to Extract Anime Info",
			Message:      "Failed to process Plex payload for: Attack on Titan\n\nError: failed to parse GUID\n\nThis indicates an issue with extracting the anime ID from the metadata. Create issue on github.com/shinkro/shinkro",
			Event:        domain.NotificationEventPlexProcessingError,
			AnimeLibrary: "Anime",
			MediaName:    "Attack on Titan",
			PlexEvent:    "media.rate",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
		{
			Subject:      "MAL Authentication Failed",
			Message:      "Failed to update MyAnimeList for: One Piece\n\nError: failed to get MAL client: token expired\n\nAction Required: Please re-authenticate with MyAnimeList in settings.",
			Event:        domain.NotificationEventAnimeUpdateError,
			AnimeLibrary: "Anime",
			MediaName:    "One Piece",
			MALID:        21,
			PlexEvent:    "media.scrobble",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
		{
			Subject:      "Mapping Not Found",
			Message:      "Failed to update MyAnimeList for: Demon Slayer\n\nError: mapping not found\n\nSource: TVDB ID 362753 (Season 2)\n\nAction Required: Add a mapping for this anime.",
			Event:        domain.NotificationEventAnimeUpdateError,
			AnimeLibrary: "Anime",
			MediaName:    "Demon Slayer",
			MALID:        38000,
			PlexEvent:    "media.scrobble",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
		{
			Subject:      "MAL API Error",
			Message:      "Failed to update MyAnimeList for: Jujutsu Kaisen\n\nError: Failed to fetch anime details from MAL API\n\nDetails: rate limit exceeded\n\nThis might be a temporary MAL API issue.",
			Event:        domain.NotificationEventAnimeUpdateError,
			AnimeLibrary: "Anime",
			MediaName:    "Jujutsu Kaisen",
			MALID:        40748,
			PlexEvent:    "media.scrobble",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
		{
			Subject:      "AniList Sync Failed",
			Message:      "Failed to sync to AniList for: Fullmetal Alchemist: Brotherhood\n\nError: anilist token expired, please re-authenticate",
			Event:        domain.NotificationEventAnimeUpdateError,
			AnimeLibrary: "Anime",
			MediaName:    "Fullmetal Alchemist: Brotherhood",
			MALID:        5114,
			AnilistID:    5114,
			PlexEvent:    "media.scrobble",
			PlexSource:   "Plex Webhook",
			Timestamp:    time.Now(),
		},
	}

	switch notification.Type {
	case domain.NotificationTypeDiscord:
		agent = NewDiscordSender(s.log, notification)
	case domain.NotificationTypeGotify:
		agent = NewGotifySender(s.log, notification)
	default:
		s.log.Error().Msgf("unsupported notification type: %v", notification.Type)
		return errors.New("unsupported notification type")
	}

	g, _ := errgroup.WithContext(ctx)

	for _, event := range events {
		e := event

		if !enabledEvent(notification.Events, e.Event) {
			continue
		}

		if err := agent.Send(e.Event, e); err != nil {
			s.log.Error().Err(err).Msgf("error sending test notification: %#v", notification)
			return err
		}

		time.Sleep(1 * time.Second)
	}

	if err := g.Wait(); err != nil {
		s.log.Error().Err(err).Msgf("Something went wrong sending test notifications to %v", notification.Type)
		return err
	}

	return nil
}

func enabledEvent(events []string, e domain.NotificationEvent) bool {
	for _, v := range events {
		if v == string(e) {
			return true
		}
		// Backward compatibility: "ERROR" matches both error types
		if v == "ERROR" {
			if e == domain.NotificationEventPlexProcessingError || e == domain.NotificationEventAnimeUpdateError {
				return true
			}
		}
	}

	return false
}
