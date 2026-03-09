package events

import (
	"context"
	"testing"
	"time"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Mock services
type mockNotificationService struct {
	sentEvents   []domain.NotificationEvent
	sentPayloads []domain.NotificationPayload
}

func (m *mockNotificationService) Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error) {
	return nil, 0, nil
}

func (m *mockNotificationService) Store(ctx context.Context, notification *domain.Notification) error {
	return nil
}

func (m *mockNotificationService) Update(ctx context.Context, notification *domain.Notification) error {
	return nil
}

func (m *mockNotificationService) Delete(ctx context.Context, id int) error {
	return nil
}

func (m *mockNotificationService) Send(event domain.NotificationEvent, payload domain.NotificationPayload) {
	m.sentEvents = append(m.sentEvents, event)
	m.sentPayloads = append(m.sentPayloads, payload)
}

func (m *mockNotificationService) Test(ctx context.Context, notification *domain.Notification) error {
	return nil
}

type mockPlexService struct {
	updateStatusErr  error
	lastPlexID       int64
	lastSuccess      *bool
	lastErrorType    string
	lastErrorMessage string
}

func (m *mockPlexService) Store(ctx context.Context, plex *domain.Plex) error {
	return nil
}

func (m *mockPlexService) Get(ctx context.Context, req *domain.GetPlexRequest) (*domain.Plex, error) {
	return nil, nil
}

func (m *mockPlexService) ProcessPlex(ctx context.Context, plex *domain.Plex) error {
	return nil
}

func (m *mockPlexService) GetPlexSettings(ctx context.Context) (*domain.PlexSettings, error) {
	return nil, nil
}

func (m *mockPlexService) CheckPlex(ctx context.Context, p *domain.Plex, ps *domain.PlexSettings) error {
	return nil
}

func (m *mockPlexService) CountScrobbleEvents(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockPlexService) CountRateEvents(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockPlexService) GetPlexHistory(ctx context.Context, limit int) ([]domain.PlexHistoryItem, error) {
	return nil, nil
}

func (m *mockPlexService) FindAllWithFilters(ctx context.Context, params domain.PlexPayloadQueryParams) (*domain.FindPlexPayloadsResponse, error) {
	return nil, nil
}

func (m *mockPlexService) Delete(ctx context.Context, req *domain.DeletePlexRequest) error {
	return nil
}

func (m *mockPlexService) UpdateStatus(ctx context.Context, plexID int64, success *bool, errorType domain.PlexErrorType, errorMessage string) error {
	m.lastPlexID = plexID
	m.lastSuccess = success
	m.lastErrorType = string(errorType)
	m.lastErrorMessage = errorMessage
	return m.updateStatusErr
}

type mockAnimeUpdateService struct {
	updateErr error
	lastAnime *domain.AnimeUpdate
	lastEvent domain.PlexEvent
}

func (m *mockAnimeUpdateService) Store(ctx context.Context, animeupdate *domain.AnimeUpdate) error {
	return nil
}

func (m *mockAnimeUpdateService) GetByID(ctx context.Context, req *domain.GetAnimeUpdateRequest) (*domain.AnimeUpdate, error) {
	return nil, nil
}

func (m *mockAnimeUpdateService) UpdateAnimeList(ctx context.Context, anime *domain.AnimeUpdate, event domain.PlexEvent) error {
	m.lastAnime = anime
	m.lastEvent = event
	return m.updateErr
}

func (m *mockAnimeUpdateService) Count(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockAnimeUpdateService) GetRecentUnique(ctx context.Context, limit int) ([]*domain.AnimeUpdate, error) {
	return nil, nil
}

func (m *mockAnimeUpdateService) GetByPlexID(ctx context.Context, plexID int64) (*domain.AnimeUpdate, error) {
	return nil, nil
}

func (m *mockAnimeUpdateService) GetByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error) {
	return nil, nil
}

func (m *mockAnimeUpdateService) FindAllWithFilters(ctx context.Context, params domain.AnimeUpdateQueryParams) (*domain.FindAnimeUpdatesResponse, error) {
	return nil, nil
}

func TestSubscriber_GetAnimeTitle(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	tests := []struct {
		name     string
		anime    *domain.AnimeUpdate
		expected string
	}{
		{
			name: "title from list details",
			anime: &domain.AnimeUpdate{
				ListDetails: domain.ListDetails{Title: "One Piece"},
			},
			expected: "One Piece",
		},
		{
			name: "title from plex movie",
			anime: &domain.AnimeUpdate{
				Plex: &domain.Plex{
					Metadata: domain.Metadata{
						Type:  "movie",
						Title: "Spirited Away",
					},
				},
			},
			expected: "Spirited Away",
		},
		{
			name: "title from plex episode grandparent",
			anime: &domain.AnimeUpdate{
				Plex: &domain.Plex{
					Metadata: domain.Metadata{
						Type:             "episode",
						GrandparentTitle: "Attack on Titan",
					},
				},
			},
			expected: "Attack on Titan",
		},
		{
			name: "title from source DB and ID",
			anime: &domain.AnimeUpdate{
				SourceDB: domain.TVDB,
				SourceId: 362753,
			},
			expected: "tvdb ID: 362753",
		},
		{
			name:     "unknown title",
			anime:    &domain.AnimeUpdate{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.getAnimeTitle(tt.anime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubscriber_GetPlexTitle(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	tests := []struct {
		name     string
		plex     *domain.Plex
		expected string
	}{
		{
			name: "movie title",
			plex: &domain.Plex{
				Metadata: domain.Metadata{
					Type:  "movie",
					Title: "Your Name",
				},
			},
			expected: "Your Name",
		},
		{
			name: "episode grandparent title",
			plex: &domain.Plex{
				Metadata: domain.Metadata{
					Type:             "episode",
					GrandparentTitle: "Demon Slayer",
				},
			},
			expected: "Demon Slayer",
		},
		{
			name: "movie without title",
			plex: &domain.Plex{
				Metadata: domain.Metadata{
					Type: "movie",
				},
			},
			expected: "Unknown",
		},
		{
			name: "episode without grandparent title",
			plex: &domain.Plex{
				Metadata: domain.Metadata{
					Type: "episode",
				},
			},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.getPlexTitle(tt.plex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubscriber_BuildErrorSubject(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	tests := []struct {
		name      string
		errorType domain.AnimeUpdateErrorType
		expected  string
	}{
		{
			name:      "MAL auth failed",
			errorType: domain.AnimeUpdateErrorMALAuthFailed,
			expected:  "MAL Authentication Failed",
		},
		{
			name:      "mapping not found",
			errorType: domain.AnimeUpdateErrorMappingNotFound,
			expected:  "Mapping Not Found",
		},
		{
			name:      "anime not in DB",
			errorType: domain.AnimeUpdateErrorAnimeNotInDB,
			expected:  "Anime Not in Database",
		},
		{
			name:      "MAL API fetch failed",
			errorType: domain.AnimeUpdateErrorMALAPIFetchFailed,
			expected:  "MAL API Error",
		},
		{
			name:      "MAL API update failed",
			errorType: domain.AnimeUpdateErrorMALAPIUpdateFailed,
			expected:  "MAL Update Failed",
		},
		{
			name:      "unknown error type",
			errorType: domain.AnimeUpdateErrorType("unknown"),
			expected:  "Anime Update Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.buildErrorSubject(tt.errorType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubscriber_BuildPlexErrorSubject(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	tests := []struct {
		name      string
		errorType domain.PlexErrorType
		expected  string
	}{
		{
			name:      "agent not supported",
			errorType: domain.PlexErrorAgentNotSupported,
			expected:  "Unsupported Metadata Agent",
		},
		{
			name:      "extraction failed",
			errorType: domain.PlexErrorExtractionFailed,
			expected:  "Failed to Extract Anime Info",
		},
		{
			name:      "unknown error type",
			errorType: domain.PlexErrorType("unknown"),
			expected:  "Plex Payload Processing Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.buildPlexErrorSubject(tt.errorType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubscriber_BuildErrorMessage(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	anime := &domain.AnimeUpdate{
		ListDetails: domain.ListDetails{Title: "Test Anime"},
		SourceDB:    domain.TVDB,
		SourceId:    362753,
		SeasonNum:   1,
	}

	tests := []struct {
		name      string
		errorType domain.AnimeUpdateErrorType
		errorMsg  string
		validate  func(*testing.T, string)
	}{
		{
			name:      "MAL auth failed",
			errorType: domain.AnimeUpdateErrorMALAuthFailed,
			errorMsg:  "invalid token",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Anime")
				assert.Contains(t, msg, "invalid token")
				assert.Contains(t, msg, "re-authenticate")
			},
		},
		{
			name:      "mapping not found",
			errorType: domain.AnimeUpdateErrorMappingNotFound,
			errorMsg:  "no mapping found",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Anime")
				assert.Contains(t, msg, "no mapping found")
				assert.Contains(t, msg, "tvdb ID 362753")
				assert.Contains(t, msg, "Season 1")
				assert.Contains(t, msg, "Add a mapping")
			},
		},
		{
			name:      "anime not in DB",
			errorType: domain.AnimeUpdateErrorAnimeNotInDB,
			errorMsg:  "not found in database",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Anime")
				assert.Contains(t, msg, "not found in database")
				assert.Contains(t, msg, "tvdb ID 362753")
			},
		},
		{
			name:      "MAL API fetch failed",
			errorType: domain.AnimeUpdateErrorMALAPIFetchFailed,
			errorMsg:  "network error",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Anime")
				assert.Contains(t, msg, "fetch anime details")
				assert.Contains(t, msg, "network error")
			},
		},
		{
			name:      "MAL API update failed",
			errorType: domain.AnimeUpdateErrorMALAPIUpdateFailed,
			errorMsg:  "update failed",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Anime")
				assert.Contains(t, msg, "update MAL list")
				assert.Contains(t, msg, "update failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.buildErrorMessage(tt.errorType, tt.errorMsg, anime)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestSubscriber_BuildPlexErrorMessage(t *testing.T) {
	subscriber := &Subscriber{
		log: zerolog.Nop(),
	}

	plex := &domain.Plex{
		Metadata: domain.Metadata{
			Type:  "movie",
			Title: "Test Movie",
		},
	}

	tests := []struct {
		name      string
		errorType domain.PlexErrorType
		errorMsg  string
		validate  func(*testing.T, string)
	}{
		{
			name:      "agent not supported",
			errorType: domain.PlexErrorAgentNotSupported,
			errorMsg:  "unsupported agent",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Movie")
				assert.Contains(t, msg, "unsupported agent")
				assert.Contains(t, msg, "supported agent")
			},
		},
		{
			name:      "extraction failed",
			errorType: domain.PlexErrorExtractionFailed,
			errorMsg:  "extraction error",
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, "Test Movie")
				assert.Contains(t, msg, "extraction error")
				assert.Contains(t, msg, "extracting the anime ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subscriber.buildPlexErrorMessage(tt.errorType, tt.errorMsg, plex)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestSubscriber_HandlePlexProcessedSuccess(t *testing.T) {
	bus := EventBus.New()
	notificationSvc := &mockNotificationService{}
	plexSvc := &mockPlexService{}
	animeUpdateSvc := &mockAnimeUpdateService{}

	_ = NewSubscribers(zerolog.Nop(), bus, notificationSvc, plexSvc, animeUpdateSvc)

	event := &domain.PlexProcessedSuccessEvent{
		PlexID:      12345,
		Plex:        testdata.NewMockPlex(),
		AnimeUpdate: testdata.NewMockAnimeUpdate(),
		Timestamp:   time.Now(),
	}

	// Publish event
	bus.Publish(domain.EventPlexProcessedSuccess, event)

	// Give goroutines time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify plex service was called
	assert.Equal(t, int64(12345), plexSvc.lastPlexID)
	assert.NotNil(t, plexSvc.lastSuccess)
	assert.True(t, *plexSvc.lastSuccess)
}

func TestSubscriber_HandlePlexProcessedError(t *testing.T) {
	bus := EventBus.New()
	notificationSvc := &mockNotificationService{}
	plexSvc := &mockPlexService{}
	animeUpdateSvc := &mockAnimeUpdateService{}

	_ = NewSubscribers(zerolog.Nop(), bus, notificationSvc, plexSvc, animeUpdateSvc)

	event := &domain.PlexProcessedErrorEvent{
		PlexID:       12345,
		Plex:         testdata.NewMockPlex(),
		ErrorType:    domain.PlexErrorAgentNotSupported,
		ErrorMessage: "unsupported agent",
		Timestamp:    time.Now(),
	}

	// Publish event
	bus.Publish(domain.EventPlexProcessedError, event)

	// Give goroutines time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify plex service was called
	assert.Equal(t, int64(12345), plexSvc.lastPlexID)
	assert.NotNil(t, plexSvc.lastSuccess)
	assert.False(t, *plexSvc.lastSuccess)
	assert.Equal(t, string(domain.PlexErrorAgentNotSupported), plexSvc.lastErrorType)
	assert.Equal(t, "unsupported agent", plexSvc.lastErrorMessage)

	// Verify notification was sent
	assert.Equal(t, 1, len(notificationSvc.sentEvents))
	assert.Equal(t, domain.NotificationEventPlexProcessingError, notificationSvc.sentEvents[0])
}

func TestSubscriber_HandleNotificationSend(t *testing.T) {
	bus := EventBus.New()
	notificationSvc := &mockNotificationService{}
	plexSvc := &mockPlexService{}
	animeUpdateSvc := &mockAnimeUpdateService{}

	_ = NewSubscribers(zerolog.Nop(), bus, notificationSvc, plexSvc, animeUpdateSvc)

	payload := domain.NotificationPayload{
		Subject: "Test Subject",
		Message: "Test Message",
	}

	event := &domain.NotificationSendEvent{
		Event:   domain.NotificationEventSuccess,
		Payload: payload,
	}

	// Publish event
	bus.Publish(domain.EventNotificationSend, event)

	// Give goroutines time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify notification was sent
	assert.Equal(t, 1, len(notificationSvc.sentEvents))
	assert.Equal(t, domain.NotificationEventSuccess, notificationSvc.sentEvents[0])
	assert.Equal(t, "Test Subject", notificationSvc.sentPayloads[0].Subject)
	assert.Equal(t, "Test Message", notificationSvc.sentPayloads[0].Message)
}

func TestSubscriber_HandleAnimeUpdateSuccess(t *testing.T) {
	bus := EventBus.New()
	notificationSvc := &mockNotificationService{}
	plexSvc := &mockPlexService{}
	animeUpdateSvc := &mockAnimeUpdateService{}

	_ = NewSubscribers(zerolog.Nop(), bus, notificationSvc, plexSvc, animeUpdateSvc)

	animeUpdate := testdata.NewMockAnimeUpdate()
	animeUpdate.ListDetails = domain.ListDetails{
		Title:           "One Piece",
		TotalEpisodeNum: 1000,
		PictureURL:      "https://example.com/pic.jpg",
	}
	animeUpdate.ListStatus.NumEpisodesWatched = 500
	animeUpdate.ListStatus.NumTimesRewatched = 2

	event := &domain.AnimeUpdateSuccessEvent{
		PlexID:      12345,
		AnimeUpdate: animeUpdate,
		Timestamp:   time.Now(),
	}

	// Publish event
	bus.Publish(domain.EventAnimeUpdateSuccess, event)

	// Give goroutines time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify notification was sent
	assert.Equal(t, 1, len(notificationSvc.sentEvents))
	assert.Equal(t, domain.NotificationEventSuccess, notificationSvc.sentEvents[0])
	assert.Equal(t, "One Piece", notificationSvc.sentPayloads[0].MediaName)
	assert.Equal(t, 500, notificationSvc.sentPayloads[0].EpisodesWatched)
}

func TestSubscriber_HandleAnimeUpdateFailed(t *testing.T) {
	bus := EventBus.New()
	notificationSvc := &mockNotificationService{}
	plexSvc := &mockPlexService{}
	animeUpdateSvc := &mockAnimeUpdateService{}

	_ = NewSubscribers(zerolog.Nop(), bus, notificationSvc, plexSvc, animeUpdateSvc)

	animeUpdate := testdata.NewMockAnimeUpdate()
	animeUpdate.Plex = testdata.NewMockPlex()

	event := &domain.AnimeUpdateFailedEvent{
		AnimeUpdate:  animeUpdate,
		ErrorType:    domain.AnimeUpdateErrorMappingNotFound,
		ErrorMessage: "mapping not found",
		Timestamp:    time.Now(),
	}

	// Publish event
	bus.Publish(domain.EventAnimeUpdateFailed, event)

	// Give goroutines time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify notification was sent
	assert.Equal(t, 1, len(notificationSvc.sentEvents))
	assert.Equal(t, domain.NotificationEventAnimeUpdateError, notificationSvc.sentEvents[0])
	assert.Contains(t, notificationSvc.sentPayloads[0].Message, "mapping not found")
}
