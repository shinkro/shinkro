package plex

import (
	"context"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"
	"github.com/shinkro/shinkro/pkg/plex"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock dependencies
type mockPlexSettingsService struct {
	settings *domain.PlexSettings
	err      error
}

func (m *mockPlexSettingsService) Get(ctx context.Context) (*domain.PlexSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.settings, nil
}

func (m *mockPlexSettingsService) Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	return nil, nil
}

func (m *mockPlexSettingsService) Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	return nil, nil
}

func (m *mockPlexSettingsService) Delete(ctx context.Context) error {
	return nil
}

func (m *mockPlexSettingsService) GetClient(ctx context.Context, ps *domain.PlexSettings) (*plex.Client, error) {
	// Return nil for testing - actual implementation would return plex.Client
	return nil, nil
}

func (m *mockPlexSettingsService) HandlePlexAgent(ctx context.Context, p *domain.Plex) (domain.PlexSupportedDBs, int, error) {
	return "", 0, nil
}

type mockPlexRepo struct{}

func (m *mockPlexRepo) Store(ctx context.Context, plex *domain.Plex) error {
	return nil
}

func (m *mockPlexRepo) FindAll(ctx context.Context) ([]*domain.Plex, error) {
	return nil, nil
}

func (m *mockPlexRepo) Get(ctx context.Context, req *domain.GetPlexRequest) (*domain.Plex, error) {
	return nil, nil
}

func (m *mockPlexRepo) CountScrobbleEvents(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockPlexRepo) CountRateEvents(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockPlexRepo) GetRecent(ctx context.Context, limit int) ([]*domain.Plex, error) {
	return nil, nil
}

func (m *mockPlexRepo) FindAllWithFilters(ctx context.Context, params domain.PlexPayloadQueryParams) (*domain.FindPlexPayloadsResponse, error) {
	return nil, nil
}

func (m *mockPlexRepo) Delete(ctx context.Context, req *domain.DeletePlexRequest) error {
	return nil
}

func (m *mockPlexRepo) UpdateStatus(ctx context.Context, plexID int64, success *bool, errorType domain.PlexErrorType, errorMsg string) error {
	return nil
}

func TestService_CheckPlex(t *testing.T) {
	bus := EventBus.New()
	service := NewService(
		zerolog.Nop(),
		&mockPlexSettingsService{},
		&mockPlexRepo{},
		nil, // anime service
		nil, // mapping service
		nil, // malauth service
		nil, // animeupdate service
		bus,
	)

	tests := []struct {
		name          string
		plex          *domain.Plex
		settings      *domain.PlexSettings
		expectedError bool
		errContains   string
	}{
		{
			name:          "valid plex payload",
			plex:          testdata.NewMockPlex(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: false,
		},
		{
			name: "unauthorized user",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Account.Title = "UnauthorizedUser"
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: true,
			errContains:   "unauthorized plex user",
		},
		{
			name: "unsupported event",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Event = domain.PlexEvent("unsupported.event")
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: true,
			errContains:   "plex event not supported",
		},
		{
			name: "non-anime library",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Metadata.LibrarySectionTitle = "Movies"
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: true,
			errContains:   "plex library not set as an anime library",
		},
		{
			name: "unsupported media type",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Metadata.Type = domain.PlexMediaType("unsupported")
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: true,
			errContains:   "plex media type not supported",
		},
		{
			name: "zero rating is allowed",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Event = domain.PlexRateEvent
				p.Rating = 0
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: false,
		},
		{
			name: "valid rate event with rating",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Event = domain.PlexRateEvent
				p.Rating = 8.5
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: false,
		},
		{
			name: "scrobble event ignores rating",
			plex: func() *domain.Plex {
				p := testdata.NewMockPlex()
				p.Event = domain.PlexScrobbleEvent
				p.Rating = 0
				return p
			}(),
			settings:      testdata.NewMockPlexSettings(),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CheckPlex(context.Background(), tt.plex, tt.settings)
			if tt.expectedError {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
