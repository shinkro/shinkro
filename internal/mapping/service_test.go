package mapping

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock repository for testing
type mockMappingRepo struct {
	settings *domain.MapSettings
	err      error
}

func (m *mockMappingRepo) Store(ctx context.Context, ms *domain.MapSettings) error {
	if m.err != nil {
		return m.err
	}
	m.settings = ms
	return nil
}

func (m *mockMappingRepo) Get(ctx context.Context) (*domain.MapSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.settings == nil {
		return &domain.MapSettings{
			TVDBEnabled:       false,
			TMDBEnabled:       false,
			CustomMapTVDBPath: "",
			CustomMapTMDBPath: "",
		}, nil
	}
	return m.settings, nil
}

func TestService_Store(t *testing.T) {
	tests := []struct {
		name          string
		settings      *domain.MapSettings
		repoError     error
		expectedError bool
		validate      func(*testing.T, *mockMappingRepo)
	}{
		{
			name: "store valid settings",
			settings: &domain.MapSettings{
				TVDBEnabled:       true,
				TMDBEnabled:       true,
				CustomMapTVDBPath: "/path/to/tvdb.yaml",
				CustomMapTMDBPath: "/path/to/tmdb.yaml",
			},
			expectedError: false,
			validate: func(t *testing.T, repo *mockMappingRepo) {
				assert.NotNil(t, repo.settings)
				assert.True(t, repo.settings.TVDBEnabled)
				assert.True(t, repo.settings.TMDBEnabled)
			},
		},
		{
			name: "store with repository error",
			settings: &domain.MapSettings{
				TVDBEnabled: true,
			},
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockMappingRepo{err: tt.repoError}
			service := NewService(zerolog.Nop(), repo)

			err := service.Store(context.Background(), tt.settings)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, repo)
				}
			}
		})
	}
}

func TestService_Get(t *testing.T) {
	tests := []struct {
		name          string
		repoSettings  *domain.MapSettings
		repoError     error
		expectedError bool
		validate      func(*testing.T, *domain.MapSettings)
	}{
		{
			name: "get existing settings",
			repoSettings: &domain.MapSettings{
				TVDBEnabled:       true,
				TMDBEnabled:       false,
				CustomMapTVDBPath: "/path/to/tvdb.yaml",
				CustomMapTMDBPath: "",
			},
			expectedError: false,
			validate: func(t *testing.T, settings *domain.MapSettings) {
				assert.True(t, settings.TVDBEnabled)
				assert.False(t, settings.TMDBEnabled)
			},
		},
		{
			name:          "get default settings when none exist",
			repoSettings:  nil,
			expectedError: false,
			validate: func(t *testing.T, settings *domain.MapSettings) {
				assert.NotNil(t, settings)
				assert.False(t, settings.TVDBEnabled)
				assert.False(t, settings.TMDBEnabled)
			},
		},
		{
			name:          "repository error",
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockMappingRepo{
				settings: tt.repoSettings,
				err:      tt.repoError,
			}
			service := NewService(zerolog.Nop(), repo)

			result, err := service.Get(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestService_CheckForAnimeinMap(t *testing.T) {
	// Create a temporary YAML file for testing
	tmpDir := t.TempDir()
	tvdbPath := filepath.Join(tmpDir, "tvdb.yaml")
	tmdbPath := filepath.Join(tmpDir, "tmdb.yaml")

	// Write test YAML files
	tvdbYAML := `AnimeMap:
  - malid: 1575
    title: "Attack on Titan"
    tvdbid: 362753
    tvdbseason: 1
    start: 0
    useMapping: false
  - malid: 21
    title: "One Piece"
    tvdbid: 81797
    tvdbseason: 0
    start: 0
    useMapping: true
    animeMapping:
      - tvdbseason: 22
        start: 892`

	tmdbYAML := `animeMovies:
  - mainTitle: "Your Name"
    tmdbid: 372058
    malid: 32281`

	err := os.WriteFile(tvdbPath, []byte(tvdbYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(tmdbPath, []byte(tmdbYAML), 0644)
	require.NoError(t, err)

	tests := []struct {
		name          string
		settings      *domain.MapSettings
		animeUpdate   *domain.AnimeUpdate
		expectedError bool
		validate      func(*testing.T, *domain.AnimeMapDetails)
	}{
		{
			name: "find TV show in map",
			settings: &domain.MapSettings{
				TVDBEnabled:       true,
				CustomMapTVDBPath: tvdbPath,
			},
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.TVDB,
				SourceId:   362753,
				SeasonNum:  1,
				EpisodeNum: 5,
				Plex:       testdata.NewMockPlex(),
			},
			expectedError: false,
			validate: func(t *testing.T, details *domain.AnimeMapDetails) {
				assert.NotNil(t, details)
				assert.Equal(t, 1575, details.Malid)
			},
		},
		{
			name: "find movie in map",
			settings: &domain.MapSettings{
				TMDBEnabled:       true,
				CustomMapTMDBPath: tmdbPath,
			},
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.TMDB,
				SourceId:   372058,
				SeasonNum:  1,
				EpisodeNum: 1,
				Plex:       testdata.NewMockPlexMovie(),
			},
			expectedError: false,
			validate: func(t *testing.T, details *domain.AnimeMapDetails) {
				assert.NotNil(t, details)
				assert.Equal(t, 32281, details.Malid)
			},
		},
		{
			name: "anime not found in map",
			settings: &domain.MapSettings{
				TVDBEnabled:       true,
				CustomMapTVDBPath: tvdbPath,
			},
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.TVDB,
				SourceId:   999999,
				SeasonNum:  1,
				EpisodeNum: 1,
				Plex:       testdata.NewMockPlex(),
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockMappingRepo{settings: tt.settings}
			service := NewService(zerolog.Nop(), repo)

			// First store the settings
			err := service.Store(context.Background(), tt.settings)
			require.NoError(t, err)

			result, err := service.CheckForAnimeinMap(context.Background(), tt.animeUpdate)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestService_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	nonExistentFile := filepath.Join(tmpDir, "notexists.txt")

	err := os.WriteFile(existingFile, []byte("test"), 0644)
	require.NoError(t, err)

	repo := &mockMappingRepo{}
	service := NewService(zerolog.Nop(), repo).(*service)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "file exists",
			path:     existingFile,
			expected: true,
		},
		{
			name:     "file does not exist",
			path:     nonExistentFile,
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.fileExists(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_CheckLocalMapsExist(t *testing.T) {
	tmpDir := t.TempDir()
	tvdbPath := filepath.Join(tmpDir, "tvdb.yaml")
	tmdbPath := filepath.Join(tmpDir, "tmdb.yaml")

	err := os.WriteFile(tvdbPath, []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(tmdbPath, []byte("test"), 0644)
	require.NoError(t, err)

	repo := &mockMappingRepo{}
	service := NewService(zerolog.Nop(), repo).(*service)

	tests := []struct {
		name         string
		settings     *domain.MapSettings
		expectedTVDB bool
		expectedTMDB bool
	}{
		{
			name: "both files exist",
			settings: &domain.MapSettings{
				CustomMapTVDBPath: tvdbPath,
				CustomMapTMDBPath: tmdbPath,
			},
			expectedTVDB: true,
			expectedTMDB: true,
		},
		{
			name: "only TVDB exists",
			settings: &domain.MapSettings{
				CustomMapTVDBPath: tvdbPath,
				CustomMapTMDBPath: filepath.Join(tmpDir, "nonexistent.yaml"),
			},
			expectedTVDB: true,
			expectedTMDB: false,
		},
		{
			name: "neither exists",
			settings: &domain.MapSettings{
				CustomMapTVDBPath: filepath.Join(tmpDir, "nonexistent1.yaml"),
				CustomMapTMDBPath: filepath.Join(tmpDir, "nonexistent2.yaml"),
			},
			expectedTVDB: false,
			expectedTMDB: false,
		},
		{
			name: "empty paths",
			settings: &domain.MapSettings{
				CustomMapTVDBPath: "",
				CustomMapTMDBPath: "",
			},
			expectedTVDB: false,
			expectedTMDB: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvdbExists, tmdbExists := service.checkLocalMapsExist(tt.settings)
			assert.Equal(t, tt.expectedTVDB, tvdbExists)
			assert.Equal(t, tt.expectedTMDB, tmdbExists)
		})
	}
}

func TestToJSONCompatible(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		validate func(*testing.T, interface{})
	}{
		{
			name:  "map with interface keys",
			input: map[interface{}]interface{}{"key": "value", 123: "number"},
			validate: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "value", m["key"])
				assert.Equal(t, "number", m["123"])
			},
		},
		{
			name:  "nested maps",
			input: map[interface{}]interface{}{"nested": map[interface{}]interface{}{"inner": "value"}},
			validate: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				nested, ok := m["nested"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "value", nested["inner"])
			},
		},
		{
			name:  "array with maps",
			input: []interface{}{map[interface{}]interface{}{"key": "value"}},
			validate: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				require.True(t, ok)
				m, ok := arr[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "value", m["key"])
			},
		},
		{
			name:  "primitive value",
			input: "string",
			validate: func(t *testing.T, result interface{}) {
				assert.Equal(t, "string", result)
			},
		},
		{
			name:  "number",
			input: 42,
			validate: func(t *testing.T, result interface{}) {
				assert.Equal(t, 42, result)
			},
		},
		{
			name:  "empty map",
			input: map[interface{}]interface{}{},
			validate: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Empty(t, m)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSONCompatible(tt.input)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
