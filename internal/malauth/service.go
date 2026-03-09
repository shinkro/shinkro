package malauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"golang.org/x/oauth2"
)

type Service interface {
	Store(ctx context.Context, ma *domain.MalAuth) error
	Get(ctx context.Context) (*domain.MalAuth, error)
	Delete(ctx context.Context) error
	GetMalClient(ctx context.Context) (*mal.Client, error)
	GetDecrypted(ctx context.Context) (*domain.MalAuth, error)
}

type service struct {
	config         *domain.Config
	log            zerolog.Logger
	repo           domain.MalAuthRepo
	tokenRefreshMu sync.Mutex // Protects token refresh to prevent concurrent refreshes
}

func NewService(config *domain.Config, log zerolog.Logger, repo domain.MalAuthRepo) Service {
	return &service{
		config: config,
		log:    log.With().Str("module", "malauth").Logger(),
		repo:   repo,
	}
}

func (s *service) Store(ctx context.Context, ma *domain.MalAuth) error {
	et, err := s.encrypt(ma.AccessToken, ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to encrypt access token")).Msg("")
		return err
	}

	ecid, err := s.encrypt([]byte(ma.Config.ClientID), ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to encrypt client id")).Msg("")
		return err
	}

	ecs, err := s.encrypt([]byte(ma.Config.ClientSecret), ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to encrypt client secret")).Msg("")
		return err
	}

	ma.Config.ClientID = string(ecid)
	ma.Config.ClientSecret = string(ecs)
	ma.AccessToken = et
	return s.repo.Store(ctx, ma)
}

func (s *service) Get(ctx context.Context) (*domain.MalAuth, error) {
	return s.repo.Get(ctx)
}

func (s *service) Delete(ctx context.Context) error {
	return s.repo.Delete(ctx)
}

func (s *service) GetDecrypted(ctx context.Context) (*domain.MalAuth, error) {
	ma, err := s.Get(ctx)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to get credentials from database")).Msg("")
		return nil, err
	}

	cid, err := s.decrypt([]byte(ma.Config.ClientID), ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to decrypt client id")).Msg("")
		return nil, err
	}

	cs, err := s.decrypt([]byte(ma.Config.ClientSecret), ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to decrypt client secret")).Msg("")
		return nil, err
	}

	ma.Config.ClientID = string(cid)
	ma.Config.ClientSecret = string(cs)
	return ma, nil
}

func (s *service) GetMalClient(ctx context.Context) (*mal.Client, error) {
	ma, err := s.GetDecrypted(ctx)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to get credentials from database")).Msg("")
		return nil, err
	}

	token := &oauth2.Token{}
	dt, err := s.decrypt(ma.AccessToken, ma.TokenIV)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to decrypt access token")).Msg("")
		return nil, err
	}

	err = json.Unmarshal(dt, token)
	if err != nil {
		s.log.Err(errors.Wrap(err, "failed to unmarshal access token")).Msg("")
		return nil, err
	}

	// Fast path: if token is still valid, return immediately without lock
	if token.Valid() {
		return mal.NewClient(ma.Config.Client(ctx, token)), nil
	}

	// Slow path: token needs refresh, acquire lock
	s.tokenRefreshMu.Lock()
	defer s.tokenRefreshMu.Unlock()

	// Double-check: another goroutine might have refreshed while we were waiting
	// Re-fetch credentials to get the latest token
	ma, err = s.GetDecrypted(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to re-fetch credentials for double-check")
	}

	dt, err = s.decrypt(ma.AccessToken, ma.TokenIV)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt access token for double-check")
	}

	var currentToken oauth2.Token
	if err = json.Unmarshal(dt, &currentToken); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal access token for double-check")
	}

	// If token is now valid (refreshed by another goroutine), use it
	if currentToken.Valid() {
		return mal.NewClient(ma.Config.Client(ctx, &currentToken)), nil
	}

	// Token still needs refresh, do it now
	freshToken, err := ma.Config.TokenSource(ctx, &currentToken).Token()
	if err != nil {
		return nil, errors.Wrap(err, "failed to refresh access token")
	}

	// Store the refreshed token
	t, err := json.Marshal(freshToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal access token")
	}

	ma.AccessToken = t
	if err = s.Store(ctx, ma); err != nil {
		return nil, errors.Wrap(err, "failed to store refreshed credentials")
	}

	return mal.NewClient(ma.Config.Client(ctx, freshToken)), nil
}

// encrypt encrypts plaintext using AES-GCM with the encryption key from config
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

	ciphertext := gcm.Seal(nil, iv, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts ciphertext using AES-GCM with the encryption key from config
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

// getEncryptionKey decodes the hex-encoded encryption key from config
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
