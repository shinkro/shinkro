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

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/oauth2"
)

type Service interface {
	Store(ctx context.Context, aa *domain.AnilistAuth) error
	Get(ctx context.Context) (*domain.AnilistAuth, error)
	Delete(ctx context.Context) error
	GetDecrypted(ctx context.Context) (*domain.AnilistAuth, error)
	GetAccessToken(ctx context.Context) (string, error)
	UpdateAnimeProgress(ctx context.Context, anilistID, progress int) error
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
		s.log.Err(errors.Wrap(err, "failed to encrypt access token")).Msg("")
		return err
	}

	ecid, err := s.encrypt([]byte(aa.Config.ClientID), aa.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to encrypt client id")).Msg("")
		return err
	}

	ecs, err := s.encrypt([]byte(aa.Config.ClientSecret), aa.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to encrypt client secret")).Msg("")
		return err
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
		s.log.Err(errors.Wrap(err, "failed to get credentials from database")).Msg("")
		return nil, err
	}

	cid, err := s.decrypt([]byte(aa.Config.ClientID), aa.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to decrypt client id")).Msg("")
		return nil, err
	}

	cs, err := s.decrypt([]byte(aa.Config.ClientSecret), aa.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to decrypt client secret")).Msg("")
		return nil, err
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

	// Fast path: token is valid
	if token.Valid() {
		return token.AccessToken, nil
	}

	// Slow path: token needs refresh
	s.tokenRefreshMu.Lock()
	defer s.tokenRefreshMu.Unlock()

	// Double-check after acquiring lock
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

	// AniList tokens don't use standard refresh flow — return error so user re-auths
	return "", errors.New("anilist token expired, please re-authenticate")
}

// UpdateAnimeProgress updates episode progress on AniList via GraphQL
func (s *service) UpdateAnimeProgress(ctx context.Context, anilistID, progress int) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

	mutation := `
mutation ($mediaId: Int, $progress: Int) {
  SaveMediaListEntry(mediaId: $mediaId, progress: $progress) {
    id
    progress
    status
  }
}
`
	variables := map[string]interface{}{
		"mediaId":  anilistID,
		"progress": progress,
	}

	return s.doGraphQL(ctx, accessToken, mutation, variables)
}

// UpdateAnimeScore updates the score on AniList via GraphQL
func (s *service) UpdateAnimeScore(ctx context.Context, anilistID int, score float64) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

	// AniList uses scores 0-100 (POINT_100) or 0-10. We receive MAL score (0-10), multiply by 10
	anilistScore := score * 10

	mutation := `
mutation ($mediaId: Int, $score: Float) {
  SaveMediaListEntry(mediaId: $mediaId, score: $score) {
    id
    score
    status
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

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
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
