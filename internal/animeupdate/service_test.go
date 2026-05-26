package animeupdate

import (
	"context"
	"errors"
	"testing"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/asaskevich/EventBus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Mock dependencies
type mockAnimeUpdateRepo struct {
	animeUpdate  *domain.AnimeUpdate
	animeUpdates []*domain.AnimeUpdate
	count        int
	err          error
}

func (m *mockAnimeUpdateRepo) Store(ctx context.Context, au *domain.AnimeUpdate) error {
	if m.err != nil {
		return m.err
	}
	m.animeUpdate = au
	return nil
}

func (m *mockAnimeUpdateRepo) GetByID(ctx context.Context, req *domain.GetAnimeUpdateRequest) (*domain.AnimeUpdate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.animeUpdate, nil
}

func (m *mockAnimeUpdateRepo) Count(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

func (m *mockAnimeUpdateRepo) GetRecentUnique(ctx context.Context, limit int) ([]*domain.AnimeUpdate, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Respect the limit
	if limit > 0 && limit < len(m.animeUpdates) {
		return m.animeUpdates[:limit], nil
	}
	return m.animeUpdates, nil
}

func (m *mockAnimeUpdateRepo) GetByPlexID(ctx context.Context, plexID int64) (*domain.AnimeUpdate, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, au := range m.animeUpdates {
		if au.PlexId == plexID {
			return au, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockAnimeUpdateRepo) GetByPlexIDs(ctx context.Context, plexIDs []int64) ([]*domain.AnimeUpdate, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := []*domain.AnimeUpdate{}
	for _, au := range m.animeUpdates {
		for _, id := range plexIDs {
			if au.PlexId == id {
				result = append(result, au)
				break
			}
		}
	}
	return result, nil
}

func (m *mockAnimeUpdateRepo) FindAllWithFilters(ctx context.Context, params domain.AnimeUpdateQueryParams) (*domain.FindAnimeUpdatesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	items := make([]domain.AnimeUpdateListItem, len(m.animeUpdates))
	for i, au := range m.animeUpdates {
		items[i] = domain.AnimeUpdateListItem{AnimeUpdate: au}
	}
	return &domain.FindAnimeUpdatesResponse{
		Data:       items,
		TotalCount: len(items),
	}, nil
}

type mockAnimeService struct {
	anime *domain.Anime
	err   error
}

func (m *mockAnimeService) GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.anime, nil
}

func (m *mockAnimeService) StoreMultiple(ctx context.Context, anime []*domain.Anime) error {
	return nil
}

func (m *mockAnimeService) GetAnime(ctx context.Context) ([]*domain.Anime, error) {
	return nil, nil
}

func (m *mockAnimeService) UpdateAnime(ctx context.Context) error {
	return nil
}

type mockMappingService struct {
	details *domain.AnimeMapDetails
	err     error
}

func (m *mockMappingService) NewMap(ctx context.Context) (*domain.AnimeMap, error) {
	return nil, nil
}

func (m *mockMappingService) Store(ctx context.Context, ms *domain.MapSettings) error {
	return nil
}

func (m *mockMappingService) Get(ctx context.Context) (*domain.MapSettings, error) {
	return nil, nil
}

func (m *mockMappingService) CheckForAnimeinMap(ctx context.Context, au *domain.AnimeUpdate) (*domain.AnimeMapDetails, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.details, nil
}

func (m *mockMappingService) ValidateMap(ctx context.Context, yamlPath string, isTVDB bool) error {
	return nil
}

type mockMALAuthService struct {
	err error
}

func (m *mockMALAuthService) Store(ctx context.Context, ma *domain.MalAuth) error {
	return nil
}

func (m *mockMALAuthService) Get(ctx context.Context) (*domain.MalAuth, error) {
	return nil, nil
}

func (m *mockMALAuthService) Delete(ctx context.Context) error {
	return nil
}

func (m *mockMALAuthService) GetMalClient(ctx context.Context) (*mal.Client, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
}

func (m *mockMALAuthService) GetDecrypted(ctx context.Context) (*domain.MalAuth, error) {
	return nil, nil
}

func TestService_Store(t *testing.T) {
	bus := EventBus.New()
	repo := &mockAnimeUpdateRepo{}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		animeUpdate   *domain.AnimeUpdate
		repoErr       error
		expectedError bool
	}{
		{
			name:          "store anime update",
			animeUpdate:   testdata.NewMockAnimeUpdate(),
			expectedError: false,
		},
		{
			name:          "repository error",
			animeUpdate:   testdata.NewMockAnimeUpdate(),
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			err := service.Store(context.Background(), tt.animeUpdate)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GetByPlexIDs(t *testing.T) {
	bus := EventBus.New()
	repo := &mockAnimeUpdateRepo{
		animeUpdates: []*domain.AnimeUpdate{
			{PlexId: 1, MALId: 1575},
			{PlexId: 2, MALId: 21},
			{PlexId: 3, MALId: 32281},
		},
	}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		plexIDs       []int64
		repoErr       error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "get by multiple plex IDs",
			plexIDs:       []int64{1, 2},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "get by single plex ID",
			plexIDs:       []int64{1},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name:          "empty plex IDs",
			plexIDs:       []int64{},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "non-existent plex IDs",
			plexIDs:       []int64{999, 1000},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "repository error",
			plexIDs:       []int64{1},
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			result, err := service.GetByPlexIDs(context.Background(), tt.plexIDs)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, len(result))
			}
		})
	}
}

func TestService_GetByPlexIDs_EmptySlice(t *testing.T) {
	bus := EventBus.New()
	repo := &mockAnimeUpdateRepo{}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	// Empty slice should return empty result without calling repo
	result, err := service.GetByPlexIDs(context.Background(), []int64{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}

func TestService_Count(t *testing.T) {
	bus := EventBus.New()
	repo := &mockAnimeUpdateRepo{count: 42}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		repoCount     int
		repoErr       error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "get count",
			repoCount:     42,
			expectedCount: 42,
			expectedError: false,
		},
		{
			name:          "zero count",
			repoCount:     0,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "repository error",
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.count = tt.repoCount
			repo.err = tt.repoErr
			result, err := service.Count(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, result)
			}
		})
	}
}

func TestService_GetByID(t *testing.T) {
	bus := EventBus.New()
	testUpdate := testdata.NewMockAnimeUpdate()
	repo := &mockAnimeUpdateRepo{animeUpdate: testUpdate}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		req           *domain.GetAnimeUpdateRequest
		repoErr       error
		expectedError bool
		validate      func(*testing.T, *domain.AnimeUpdate)
	}{
		{
			name:          "get by ID",
			req:           &domain.GetAnimeUpdateRequest{Id: 1},
			expectedError: false,
			validate: func(t *testing.T, au *domain.AnimeUpdate) {
				assert.NotNil(t, au)
				assert.Equal(t, testUpdate.MALId, au.MALId)
			},
		},
		{
			name:          "repository error",
			req:           &domain.GetAnimeUpdateRequest{Id: 1},
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			result, err := service.GetByID(context.Background(), tt.req)
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

func TestService_ConvertAniDBToTVDB(t *testing.T) {
	bus := EventBus.New()
	animeService := &mockAnimeService{
		anime: &domain.Anime{
			TVDBId: 362753,
		},
	}
	service := NewService(
		zerolog.Nop(),
		&mockAnimeUpdateRepo{},
		animeService,
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	).(*service)

	tests := []struct {
		name        string
		animeUpdate *domain.AnimeUpdate
		anime       *domain.Anime
		animeError  error
		expectedDB  domain.PlexSupportedDBs
		expectedID  int
	}{
		{
			name: "convert AniDB to TVDB",
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.AniDB,
				SourceId:   12345,
				EpisodeNum: 5,
			},
			anime: &domain.Anime{
				TVDBId: 362753,
			},
			expectedDB: domain.TVDB,
			expectedID: 362753,
		},
		{
			name: "no conversion for non-AniDB source",
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.TVDB,
				SourceId:   362753,
				EpisodeNum: 5,
			},
			expectedDB: domain.TVDB,
			expectedID: 362753,
		},
		{
			name: "no conversion when anime not found",
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.AniDB,
				SourceId:   12345,
				EpisodeNum: 5,
			},
			animeError: errors.New("not found"),
			expectedDB: domain.AniDB,
			expectedID: 12345,
		},
		{
			name: "no conversion when TVDBId is 0",
			animeUpdate: &domain.AnimeUpdate{
				SourceDB:   domain.AniDB,
				SourceId:   12345,
				EpisodeNum: 5,
			},
			anime: &domain.Anime{
				TVDBId: 0,
			},
			expectedDB: domain.AniDB,
			expectedID: 12345,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			animeService.anime = tt.anime
			animeService.err = tt.animeError

			result := service.convertAniDBToTVDB(context.Background(), tt.animeUpdate)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedDB, result.SourceDB)
			assert.Equal(t, tt.expectedID, result.SourceId)
		})
	}
}

func TestService_GetRecentUnique(t *testing.T) {
	bus := EventBus.New()
	updates := []*domain.AnimeUpdate{
		testdata.NewMockAnimeUpdate(),
		testdata.NewMockAnimeUpdateFirstEpisode(),
	}
	repo := &mockAnimeUpdateRepo{animeUpdates: updates}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		limit         int
		repoErr       error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "get recent with limit",
			limit:         5,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "get recent with limit less than available",
			limit:         1,
			expectedCount: 1,
			expectedError: false,
		},
		{
			name:          "repository error",
			limit:         5,
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			result, err := service.GetRecentUnique(context.Background(), tt.limit)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectedCount > 0 {
					assert.LessOrEqual(t, len(result), tt.expectedCount)
				}
			}
		})
	}
}

func TestService_FindAllWithFilters(t *testing.T) {
	bus := EventBus.New()
	updates := []*domain.AnimeUpdate{
		testdata.NewMockAnimeUpdate(),
		testdata.NewMockAnimeUpdateWithStatus(domain.AnimeUpdateStatusFailed, domain.AnimeUpdateErrorMappingNotFound),
	}
	repo := &mockAnimeUpdateRepo{animeUpdates: updates}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		params        domain.AnimeUpdateQueryParams
		repoErr       error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "find all with filters",
			params:        domain.AnimeUpdateQueryParams{},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "repository error",
			params:        domain.AnimeUpdateQueryParams{},
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			result, err := service.FindAllWithFilters(context.Background(), tt.params)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, result.TotalCount)
			}
		})
	}
}

func TestService_GetByPlexID(t *testing.T) {
	bus := EventBus.New()
	testUpdate := testdata.NewMockAnimeUpdate()
	testUpdate.PlexId = 12345
	repo := &mockAnimeUpdateRepo{
		animeUpdates: []*domain.AnimeUpdate{testUpdate},
	}
	service := NewService(
		zerolog.Nop(),
		repo,
		&mockAnimeService{},
		&mockMappingService{},
		&mockMALAuthService{},
		nil,
		bus,
	)

	tests := []struct {
		name          string
		plexID        int64
		repoErr       error
		expectedError bool
		validate      func(*testing.T, *domain.AnimeUpdate)
	}{
		{
			name:          "get by plex ID",
			plexID:        12345,
			expectedError: false,
			validate: func(t *testing.T, au *domain.AnimeUpdate) {
				assert.NotNil(t, au)
				assert.Equal(t, int64(12345), au.PlexId)
			},
		},
		{
			name:          "plex ID not found",
			plexID:        99999,
			expectedError: true,
		},
		{
			name:          "repository error",
			plexID:        12345,
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoErr
			result, err := service.GetByPlexID(context.Background(), tt.plexID)
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
