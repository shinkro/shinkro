package server

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type mockAnimeService struct {
	updateErr   error
	updateCount int
}

func (m *mockAnimeService) GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error) {
	return nil, nil
}

func (m *mockAnimeService) StoreMultiple(ctx context.Context, anime []*domain.Anime) error {
	return nil
}

func (m *mockAnimeService) GetAnime(ctx context.Context) ([]*domain.Anime, error) {
	return nil, nil
}

func (m *mockAnimeService) UpdateAnime(ctx context.Context) error {
	m.updateCount++
	return m.updateErr
}

type mockMappingService struct {
	getErr      error
	storeErr    error
	getCalled   bool
	storeCalled bool
}

func (m *mockMappingService) NewMap(ctx context.Context) (*domain.AnimeMap, error) {
	return nil, nil
}

func (m *mockMappingService) Store(ctx context.Context, ms *domain.MapSettings) error {
	m.storeCalled = true
	return m.storeErr
}

func (m *mockMappingService) Get(ctx context.Context) (*domain.MapSettings, error) {
	m.getCalled = true
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &domain.MapSettings{}, nil
}

func (m *mockMappingService) CheckForAnimeinMap(ctx context.Context, au *domain.AnimeUpdate) (*domain.AnimeMapDetails, error) {
	return nil, nil
}

func (m *mockMappingService) ValidateMap(ctx context.Context, yamlPath string, isTVDB bool) error {
	return nil
}

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{
			name:     "newer version available",
			current:  "v1.0.0",
			latest:   "v1.1.0",
			expected: true,
		},
		{
			name:     "same version",
			current:  "v1.0.0",
			latest:   "v1.0.0",
			expected: false,
		},
		{
			name:     "older version (downgrade)",
			current:  "v1.1.0",
			latest:   "v1.0.0",
			expected: false,
		},
		{
			name:     "minor version update",
			current:  "v1.0.0",
			latest:   "v1.0.1",
			expected: true,
		},
		{
			name:     "major version update",
			current:  "v1.0.0",
			latest:   "v2.0.0",
			expected: true,
		},
		{
			name:     "patch version update",
			current:  "v1.0.0",
			latest:   "v1.0.1",
			expected: true,
		},
		{
			name:     "without v prefix",
			current:  "1.0.0",
			latest:   "1.1.0",
			expected: true,
		},
		{
			name:     "mixed prefix",
			current:  "v1.0.0",
			latest:   "1.1.0",
			expected: true,
		},
		{
			name:     "longer version current",
			current:  "v1.0.0.0",
			latest:   "v1.0.1",
			expected: true,
		},
		{
			name:     "longer version latest",
			current:  "v1.0.0",
			latest:   "v1.0.1.0",
			expected: true,
		},
		{
			name:     "version with non-numeric suffix",
			current:  "v1.0.0",
			latest:   "v1.0.1-beta",
			expected: true,
		},
		{
			name:     "version with non-numeric prefix",
			current:  "v1.0.0-alpha",
			latest:   "v1.0.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUpdateAvailable(tt.current, tt.latest)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_Start(t *testing.T) {
	bus := EventBus.New()
	animeSvc := &mockAnimeService{}
	mappingSvc := &mockMappingService{}

	config := &domain.Config{
		CheckForUpdates: false,
		Version:         "v1.0.0",
	}

	server := NewServer(zerolog.Nop(), config, animeSvc, mappingSvc, bus)

	tests := []struct {
		name          string
		updateErr     error
		getErr        error
		expectedError bool
		validate      func(*testing.T)
	}{
		{
			name:          "successful start",
			expectedError: false,
			validate: func(t *testing.T) {
				assert.Equal(t, 1, animeSvc.updateCount)
				assert.True(t, mappingSvc.getCalled)
			},
		},
		{
			name:          "anime update error",
			updateErr:     errors.New("update failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			animeSvc.updateErr = tt.updateErr
			mappingSvc.getErr = tt.getErr
			animeSvc.updateCount = 0
			mappingSvc.getCalled = false
			mappingSvc.storeCalled = false

			err := server.Start()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t)
				}
			}
		})
	}
}

func TestServer_Start_InitializesMapSettings(t *testing.T) {
	bus := EventBus.New()
	animeSvc := &mockAnimeService{}
	mappingSvc := &mockMappingService{
		getErr: sql.ErrNoRows, // Use actual sql.ErrNoRows
	}

	config := &domain.Config{
		CheckForUpdates: false,
		Version:         "v1.0.0",
	}

	server := NewServer(zerolog.Nop(), config, animeSvc, mappingSvc, bus)

	err := server.Start()
	assert.NoError(t, err)

	// Verify that Store was called to initialize map settings
	assert.True(t, mappingSvc.storeCalled)
}

func TestServer_CheckAndNotifyUpdate(t *testing.T) {
	bus := EventBus.New()
	animeSvc := &mockAnimeService{}
	mappingSvc := &mockMappingService{}

	tests := []struct {
		name              string
		checkForUpdates   bool
		version           string
		latestTag         string
		latestTagErr      error
		expectedPublished bool
	}{
		{
			name:              "updates disabled",
			checkForUpdates:   false,
			version:           "v1.0.0",
			expectedPublished: false,
		},
		{
			name:              "dev version",
			checkForUpdates:   true,
			version:           "dev",
			expectedPublished: false,
		},
		{
			name:              "nightly version",
			checkForUpdates:   true,
			version:           "nightly",
			expectedPublished: false,
		},
		{
			name:              "empty version",
			checkForUpdates:   true,
			version:           "",
			expectedPublished: false,
		},
		{
			name:              "no update available",
			checkForUpdates:   true,
			version:           "v1.1.0",
			latestTag:         "v1.0.0",
			expectedPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &domain.Config{
				CheckForUpdates: tt.checkForUpdates,
				Version:         tt.version,
			}

			server := NewServer(zerolog.Nop(), config, animeSvc, mappingSvc, bus)
			server.lastUpdateNotified = ""

			// We can't easily test the actual update check without mocking update.LatestTag
			// But we can verify the function doesn't panic
			assert.NotPanics(t, func() {
				server.checkAndNotifyUpdate()
			})
		})
	}
}
