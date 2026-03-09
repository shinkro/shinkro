package anime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/sharedhttp"
)

type Service interface {
	GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error)
	StoreMultiple(ctx context.Context, anime []*domain.Anime) error
	GetAnime(ctx context.Context) ([]*domain.Anime, error)
	UpdateAnime(ctx context.Context) error
}

type service struct {
	log  zerolog.Logger
	repo domain.AnimeRepo
}

func NewService(log zerolog.Logger, repo domain.AnimeRepo) Service {
	return &service{
		log:  log.With().Str("module", "anime").Logger(),
		repo: repo,
	}
}

func (s *service) GetByID(ctx context.Context, req *domain.GetAnimeRequest) (*domain.Anime, error) {
	return s.repo.GetByID(ctx, req)
}

func (s *service) StoreMultiple(ctx context.Context, anime []*domain.Anime) error {
	return s.repo.StoreMultiple(anime)
}

// GetAnime fetches anime data from the external ShinkroDB API
func (s *service) GetAnime(ctx context.Context) ([]*domain.Anime, error) {
	client := &http.Client{Transport: sharedhttp.Transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(domain.ShinkroDB), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request to shinkrodb")
	}
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response from shinkrodb")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	a := []*domain.Anime{}
	err = json.Unmarshal(body, &a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json")
	}

	return a, nil
}

func (s *service) UpdateAnime(ctx context.Context) error {
	s.log.Info().Msg("Started anime update in database.")
	anime, err := s.GetAnime(ctx)
	if err != nil {
		return err
	}

	err = s.StoreMultiple(ctx, anime)
	if err != nil {
		return err
	}

	s.log.Info().
		Int("animeCount", len(anime)).
		Msg("Anime update complete.")

	return nil
}
