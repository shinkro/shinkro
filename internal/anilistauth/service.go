package anilistauth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/oauth2"
)

// AnilistStatus mirrors MAL statuses for AniList
type AnilistStatus string

const (
	AnilistStatusCurrent   AnilistStatus = "CURRENT"
	AnilistStatusCompleted AnilistStatus = "COMPLETED"
	AnilistStatusRepeating AnilistStatus = "REPEATING"
)

// AnilistUpdateParams holds all fields to sync to AniList in one call
type AnilistUpdateParams struct {
	AnilistID   int
	Progress    int
	Status      AnilistStatus
	Score       float64 // 0 means no score update
	StartedAt   *time.Time
	CompletedAt *time.Time
	Repeat      int // number of rewatches
}

type Service interface {
	Store(ctx context.Context, aa *domain.AnilistAuth) error
	Get(ctx context.Context) (*domain.AnilistAuth, error)
	Delete(ctx context.Context) error
	GetDecrypted(ctx context.Context) (*domain.AnilistAuth, error)
	GetAccessToken(ctx context.Context) (string, error)
	UpdateAnimeEntry(ctx context.Context, params AnilistUpdateParams) error
	UpdateAnimeScore(ctx context.Context, anilistID int, score float64) error
}

type service struct {
	config         *domain.Config
	log            zerolog.Logger
	repo           domain.AnilistAuthRepo
	tokenRefreshMu sync.Mutex
}

func NewService(config *domain.Config, log zerolog.Logger, repo domain.AnilistAuthRepo) Service {
	return &service{
		config: config,
		log:    log.With().Str("module", "anilistauth").Logger(),
		repo:   repo,
	}
}

func (s *service) Store(ctx context.Context, aa *domain.AnilistAuth) error {
	et, err := s.encrypt(aa.AccessToken, aa.TokenIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt access token")
	}
	ecid, err := s.encrypt([]byte(aa.Config.ClientID), aa.TokenIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt client id")
	}
	ecs, err := s.encrypt([]byte(aa.Config.ClientSecret), aa.TokenIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt client secret")
	}
	aa.Config.ClientID = string(ecid)
	aa.Config.ClientSecret = string(ecs)
	aa.AccessToken = et
	return s.repo.Store(ctx, aa)
}

func (s *service) Get(ctx context.Context) (*domain.AnilistAuth, error) {
	return s.repo.Get(ctx)
}

func (s *service) Delete(ctx context.Context) error {
	return s.repo.Delete(ctx)
}

func (s *service) GetDecrypted(ctx context.Context) (*domain.AnilistAuth, error) {
	aa, err := s.Get(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get credentials from database")
	}
	cid, err := s.decrypt([]byte(aa.Config.ClientID), aa.TokenIV)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt client id")
	}
	cs, err := s.decrypt([]byte(aa.Config.ClientSecret), aa.TokenIV)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt client secret")
	}
	aa.Config.ClientID = string(cid)
	aa.Config.ClientSecret = string(cs)
	return aa, nil
}

func (s *service) GetAccessToken(ctx context.Context) (string, error) {
	aa, err := s.GetDecrypted(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get credentials")
	}

	token := &oauth2.Token{}
	dt, err := s.decrypt(aa.AccessToken, aa.TokenIV)
	if err != nil {
		return "", errors.Wrap(err, "failed to decrypt access token")
	}
	if err = json.Unmarshal(dt, token); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal access token")
	}
	if token.Valid() {
		return token.AccessToken, nil
	}

	s.tokenRefreshMu.Lock()
	defer s.tokenRefreshMu.Unlock()

	aa, err = s.GetDecrypted(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to re-fetch credentials")
	}
	dt, err = s.decrypt(aa.AccessToken, aa.TokenIV)
	if err != nil {
		return "", errors.Wrap(err, "failed to decrypt access token on recheck")
	}
	var currentToken oauth2.Token
	if err = json.Unmarshal(dt, &currentToken); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal access token on recheck")
	}
	if currentToken.Valid() {
		return currentToken.AccessToken, nil
	}
	return "", errors.New("anilist token expired, please re-authenticate")
}

// UpdateAnimeEntry updates all relevant fields on AniList in a single GraphQL mutation.
// This mirrors exactly what shinkro does with MAL: progress, status, start date,
// finish date, and rewatch count.
func (s *service) UpdateAnimeEntry(ctx context.Context, params AnilistUpdateParams) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

	// Build variables dynamically — only include fields that are set
	variables := map[string]interface{}{
		"mediaId":  params.AnilistID,
		"progress": params.Progress,
		"status":   string(params.Status),
	}

	if params.Repeat > 0 {
		variables["repeat"] = params.Repeat
	}
	if params.StartedAt != nil {
		t := params.StartedAt
		variables["startedAt"] = map[string]interface{}{
			"year":  t.Year(),
			"month": int(t.Month()),
			"day":   t.Day(),
		}
	}
	if params.CompletedAt != nil {
		t := params.CompletedAt
		variables["completedAt"] = map[string]interface{}{
			"year":  t.Year(),
			"month": int(t.Month()),
			"day":   t.Day(),
		}
	}

	mutation := `
mutation (
  $mediaId: Int,
  $progress: Int,
  $status: MediaListStatus,
  $repeat: Int,
  $startedAt: FuzzyDateInput,
  $completedAt: FuzzyDateInput
) {
  SaveMediaListEntry(
    mediaId: $mediaId,
    progress: $progress,
    status: $status,
    repeat: $repeat,
    startedAt: $startedAt,
    completedAt: $completedAt
  ) {
    id
    progress
    status
    repeat
    startedAt { year month day }
    completedAt { year month day }
  }
}
`
	return s.doGraphQL(ctx, accessToken, mutation, variables)
}

// UpdateAnimeScore updates only the score on AniList (used for rate events).
// AniList score is 0-100 (POINT_100 format); MAL score is 0-10, so we multiply by 10.
func (s *service) UpdateAnimeScore(ctx context.Context, anilistID int, score float64) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

	anilistScore := score * 10
	mutation := `
mutation ($mediaId: Int, $score: Float) {
  SaveMediaListEntry(mediaId: $mediaId, score: $score) {
    id
    score
  }
}
`
	variables := map[string]interface{}{
		"mediaId": anilistID,
		"score":   anilistScore,
	}
	return s.doGraphQL(ctx, accessToken, mutation, variables)
}

func (s *service) doGraphQL(ctx context.Context, accessToken string, query string, variables map[string]interface{}) error {
	body, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal graphql request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, domain.AnilistGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "failed to create graphql request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute graphql request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read graphql response")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("anilist graphql error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err = json.Unmarshal(respBody, &result); err != nil {
		return errors.Wrap(err, "failed to parse graphql response")
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("anilist graphql error: %s", result.Errors[0].Message)
	}
	return nil
}

func (s *service) encrypt(plaintext, iv []byte) ([]byte, error) {
	key, err := s.getEncryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, iv, plaintext, nil), nil
}

func (s *service) decrypt(ciphertext, iv []byte) ([]byte, error) {
	key, err := s.getEncryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ciphertext, nil)
}

func (s *service) getEncryptionKey() ([]byte, error) {
	key, err := hex.DecodeString(s.config.EncryptionKey)
	if err != nil {
		return nil, errors.New("invalid hex encryption key")
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	return key, nil
}
