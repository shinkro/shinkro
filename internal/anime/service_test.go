package anime

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type mockAnimeRepo struct {
	anime        *domain.Anime
	animeList    []*domain.Anime
	storeErr     error
	getByIDErr   error
	replaceCalls int
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

func (m *mockAnimeRepo) ReplaceAll(ctx context.Context, anime []*domain.Anime) error {
	m.replaceCalls++
	return m.StoreMultiple(anime)
}

type snapshotTransport func(*http.Request) (*http.Response, error)

func (f snapshotTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUpdateAnimeReplacesOnlyAValidDownloadedSnapshot(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		valid  bool
	}{
		{"valid", 200, `[{"malid":1,"title":"Live"}]`, true},
		{"server error with JSON array", 503, `[{"malid":1,"title":"Live"}]`, false},
		{"empty", 200, `[]`, false},
		{"null", 200, `null`, false},
		{"null entry", 200, `[null]`, false},
		{"invalid ID", 200, `[{"malid":0,"title":"Invalid"}]`, false},
		{"missing title", 200, `[{"malid":1}]`, false},
		{"truncated JSON", 200, `[{"malid":1`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockAnimeRepo{animeList: []*domain.Anime{{MALId: 9, MainTitle: "Previous"}}}
			s := NewService(zerolog.Nop(), repo).(*service)
			s.httpClient.Transport = snapshotTransport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})
			err := s.UpdateAnime(context.Background())
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, 1, repo.replaceCalls)
				assert.Equal(t, 1, repo.animeList[0].MALId)
			} else {
				assert.Error(t, err)
				assert.Zero(t, repo.replaceCalls)
				assert.Equal(t, 9, repo.animeList[0].MALId)
			}
		})
	}
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
