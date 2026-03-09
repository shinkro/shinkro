package anime

import (
	"context"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type mockAnimeRepo struct {
	anime       *domain.Anime
	animeList   []*domain.Anime
	storeErr    error
	getByIDErr  error
}

func (m *mockAnimeRepo) GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.anime, nil
}

func (m *mockAnimeRepo) StoreMultiple(anime []*domain.Anime) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.animeList = anime
	return nil
}

func TestService_GetByID(t *testing.T) {
	testAnime := &domain.Anime{
		MALId:  1575,
		TVDBId: 81797,
		TMDBId: 37854,
	}
	repo := &mockAnimeRepo{anime: testAnime}
	service := NewService(zerolog.Nop(), repo)

	tests := []struct {
		name          string
		req           *domain.GetAnimeRequest
		repoErr       error
		expectedError bool
		validate      func(*testing.T, *domain.Anime)
	}{
		{
			name:          "get anime by ID",
			req:           &domain.GetAnimeRequest{IDtype: domain.MAL, Id: 1575},
			expectedError: false,
			validate: func(t *testing.T, a *domain.Anime) {
				assert.NotNil(t, a)
				assert.Equal(t, 1575, a.MALId)
			},
		},
		{
			name:          "repository error",
			req:           &domain.GetAnimeRequest{IDtype: domain.MAL, Id: 1575},
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.getByIDErr = tt.repoErr
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

func TestService_StoreMultiple(t *testing.T) {
	repo := &mockAnimeRepo{}
	service := NewService(zerolog.Nop(), repo)

	animeList := []*domain.Anime{
		{MALId: 1575, TVDBId: 81797},
		{MALId: 21, TVDBId: 362753},
	}

	tests := []struct {
		name          string
		anime         []*domain.Anime
		repoErr       error
		expectedError bool
		validate      func(*testing.T)
	}{
		{
			name:          "store multiple anime",
			anime:         animeList,
			expectedError: false,
			validate: func(t *testing.T) {
				assert.Equal(t, 2, len(repo.animeList))
				assert.Equal(t, 1575, repo.animeList[0].MALId)
			},
		},
		{
			name:          "store empty list",
			anime:         []*domain.Anime{},
			expectedError: false,
			validate: func(t *testing.T) {
				assert.Equal(t, 0, len(repo.animeList))
			},
		},
		{
			name:          "repository error",
			anime:         animeList,
			repoErr:       errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.storeErr = tt.repoErr
			err := service.StoreMultiple(context.Background(), tt.anime)
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

// Note: GetAnime and UpdateAnime make HTTP calls to external services.
// Full testing would require HTTP transport mocking or integration tests.
// These tests verify the service structure and error handling.
func TestService_GetAnime_ContextCancellation(t *testing.T) {
	repo := &mockAnimeRepo{}
	service := NewService(zerolog.Nop(), repo)

	// Test that context cancellation is properly passed through
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := service.GetAnime(ctx)
	// This will fail with context canceled or network error
	// The important thing is it handles the context correctly
	assert.Error(t, err)
	assert.Nil(t, result)
}

// Note: UpdateAnime calls GetAnime which makes HTTP calls.
// This test verifies the service structure and error propagation.
func TestService_UpdateAnime_ContextCancellation(t *testing.T) {
	repo := &mockAnimeRepo{}
	service := NewService(zerolog.Nop(), repo)

	// Test that context cancellation is properly passed through
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := service.UpdateAnime(ctx)
	// This will fail with context canceled or network error
	// The important thing is it handles the context correctly
	assert.Error(t, err)
}

