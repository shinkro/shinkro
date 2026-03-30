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

// AnilistUpdateParams holds all fields to sync to AniList in one call.
type AnilistUpdateParams struct {
	AnilistID   int
	Progress    int
	Status      AnilistStatus
	Score       float64    // 0 means no score update
	StartedAt   *time.Time
	CompletedAt *time.Time
	Repeat      int // number of rewatches
}

// AnilistEntryDates holds existing start/completion dates from AniList,
// used to avoid overwriting dates that are already set.
type AnilistEntryDates struct {
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type Service interface {
	Store(ctx context.Context, aa *domain.AnilistAuth) error
	Get(ctx context.Context) (*domain.AnilistAuth, error)
	Delete(ctx context.Context) error
	GetDecrypted(ctx context.Context) (*domain.AnilistAuth, error)
	GetAccessToken(ctx context.Context) (string, error)
	GetAnimeEntry(ctx context.Context, anilistID int) (*AnilistEntryDates, error)
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

// Store encrypts credentials before persisting them to the database.
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

// GetDecrypted returns the AnilistAuth with all encrypted fields decrypted in-place.
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

// GetAccessToken returns a valid access token, returning an error if the token
// is expired (AniList tokens do not support refresh).
// Uses a mutex to avoid concurrent re-authentication races.
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

	// Token may have been refreshed by another goroutine — re-check under lock.
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

// GetAnimeEntry fetches the existing start/completion dates for an anime entry.
// Returns empty dates (not nil) when the entry does not exist on AniList yet.
func (s *service) GetAnimeEntry(ctx context.Context, anilistID int) (*AnilistEntryDates, error) {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get anilist access token")
	}

	query := `
query ($mediaId: Int) {
  MediaList(mediaId: $mediaId, type: ANIME) {
    startedAt { year month day }
    completedAt { year month day }
  }
}`
	variables := map[string]interface{}{"mediaId": anilistID}

	type fuzzyDate struct {
		Year  int `json:"year"`
		Month int `json:"month"`
		Day   int `json:"day"`
	}
	var result struct {
		Data struct {
			MediaList struct {
				StartedAt   fuzzyDate `json:"startedAt"`
				CompletedAt fuzzyDate `json:"completedAt"`
			} `json:"MediaList"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := s.doGraphQL(ctx, accessToken, query, variables, &result); err != nil {
		return nil, err
	}
	if len(result.Errors) > 0 {
		// Entry doesn't exist on AniList yet — return empty dates, not an error.
		return &AnilistEntryDates{}, nil
	}

	dates := &AnilistEntryDates{}
	if fd := result.Data.MediaList.StartedAt; fd.Year > 0 {
		t := time.Date(fd.Year, time.Month(fd.Month), fd.Day, 0, 0, 0, 0, time.UTC)
		dates.StartedAt = &t
	}
	if fd := result.Data.MediaList.CompletedAt; fd.Year > 0 {
		t := time.Date(fd.Year, time.Month(fd.Month), fd.Day, 0, 0, 0, 0, time.UTC)
		dates.CompletedAt = &t
	}
	return dates, nil
}

// UpdateAnimeEntry syncs all relevant fields to AniList in a single GraphQL mutation.
// This mirrors exactly what shinkro does with MAL: progress, status, start/finish
// dates, and rewatch count.
func (s *service) UpdateAnimeEntry(ctx context.Context, params AnilistUpdateParams) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

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
	return s.doGraphQL(ctx, accessToken, mutation, variables, nil)
}

// UpdateAnimeScore updates only the score on AniList (used for rate events).
// AniList uses POINT_100 format (0–100); MAL uses 0–10, so we multiply by 10.
func (s *service) UpdateAnimeScore(ctx context.Context, anilistID int, score float64) error {
	accessToken, err := s.GetAccessToken(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get anilist access token")
	}

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
		"score":   score * 10,
	}
	return s.doGraphQL(ctx, accessToken, mutation, variables, nil)
}

// doGraphQL executes a GraphQL request against the AniList API.
// If result is non-nil, the response body is unmarshalled into it.
func (s *service) doGraphQL(ctx context.Context, accessToken, query string, variables map[string]interface{}, result interface{}) error {
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

	// Always check for GraphQL-level errors.
	var errCheck struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err = json.Unmarshal(respBody, &errCheck); err != nil {
		return errors.Wrap(err, "failed to parse graphql response")
	}
	if len(errCheck.Errors) > 0 {
		return fmt.Errorf("anilist graphql error: %s", errCheck.Errors[0].Message)
	}

	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

// ── Encryption helpers ────────────────────────────────────────────────────────

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
