package anime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

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
	log        zerolog.Logger
	repo       domain.AnimeRepo
	httpClient *http.Client
}

func NewService(log zerolog.Logger, repo domain.AnimeRepo) Service {
	return &service{
		log:        log.With().Str("module", "anime").Logger(),
		repo:       repo,
		httpClient: &http.Client{Transport: sharedhttp.Transport, Timeout: 30 * time.Second},
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(domain.ShinkroDB), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request to shinkrodb")
	}
	req.Header.Set("User-Agent", sharedhttp.UserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response from shinkrodb")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected ShinkroDB status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	a := []*domain.Anime{}
	err = json.Unmarshal(body, &a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json")
	}
	if len(a) == 0 {
		return nil, errors.New("ShinkroDB catalogue is empty")
	}
	for _, row := range a {
		if row == nil || row.MALId <= 0 || strings.TrimSpace(row.MainTitle) == "" {
			return nil, errors.New("ShinkroDB catalogue contains an invalid entry")
		}
	}

	return a, nil
}

func (s *service) UpdateAnime(ctx context.Context) error {
	s.log.Info().Msg("Started anime update in database.")
	anime, err := s.GetAnime(ctx)
	if err != nil {
		return err
	}

	err = s.repo.ReplaceAll(ctx, anime)
	if err != nil {
		return err
	}

	s.log.Info().
		Int("animeCount", len(anime)).
		Msg("Anime update complete.")

	return nil
}
