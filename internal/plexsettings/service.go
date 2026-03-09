package plexsettings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dcarbone/zadapters/zstdlog"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/plex"
)

type Service interface {
	Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error)
	Get(ctx context.Context) (*domain.PlexSettings, error)
	Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error)
	Delete(ctx context.Context) error
	GetClient(ctx context.Context, ps *domain.PlexSettings) (*plex.Client, error)
	HandlePlexAgent(ctx context.Context, p *domain.Plex) (domain.PlexSupportedDBs, int, error)
}

type service struct {
	config *domain.Config
	log    zerolog.Logger
	repo   domain.PlexSettingsRepo
}

func NewService(config *domain.Config, log zerolog.Logger, repo domain.PlexSettingsRepo) Service {
	return &service{
		config: config,
		log:    log.With().Str("module", "plexsettings").Logger(),
		repo:   repo,
	}
}

func (s *service) Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	eToken, err := s.encrypt(ps.Token, ps.TokenIV)
	if err != nil {
		s.log.Error().Err(err).Msg("error encrypting token")
		return nil, err
	}

	ps.Token = eToken
	return s.repo.Store(ctx, ps)
}

func (s *service) Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	return s.repo.Update(ctx, ps)
}

func (s *service) Get(ctx context.Context) (*domain.PlexSettings, error) {
	return s.repo.Get(ctx)
}

func (s *service) Delete(ctx context.Context) error {
	return s.repo.Delete(ctx)
}

func (s *service) GetClient(ctx context.Context, ps *domain.PlexSettings) (*plex.Client, error) {
	if len(ps.TokenIV) == 0 {
		tempPs, err := s.repo.Get(ctx)
		if err != nil {
			s.log.Error().Err(err).Msg("error getting plex settings")
			return nil, err
		}
		ps.Token = tempPs.Token
		ps.TokenIV = tempPs.TokenIV
		s.log.Trace().Msg("loaded token and tokenIV from database")
	}

	scheme := "http"
	if ps.TLS {
		scheme = "https"
	}

	if len(ps.Token) == 0 || len(ps.TokenIV) == 0 {
		return nil, errors.New("token or tokenIV is empty")
	}

	token, err := s.decrypt(ps.Token, ps.TokenIV)
	if err != nil {
		return nil, err
	}

	c := plex.NewClient(plex.Config{
		Url:           fmt.Sprintf("%s://%s:%d", scheme, ps.Host, ps.Port),
		Token:         string(token),
		ClientID:      ps.ClientID,
		TLSSkipVerify: ps.TLSSkip,
		Log:           zstdlog.NewStdLoggerWithLevel(s.log.With().Str("client", "plex").Logger(), zerolog.TraceLevel),
	})

	return c, nil
}

func (s *service) HandlePlexAgent(ctx context.Context, p *domain.Plex) (domain.PlexSupportedDBs, int, error) {
	if p.Metadata.Type == domain.PlexEpisode {
		ps, err := s.repo.Get(ctx)
		if err != nil {
			return "", 0, err
		}

		pc, err := s.GetClient(ctx, ps)
		if err != nil {
			return "", 0, err
		}

		guid, err := pc.GetShowID(ctx, p.Metadata.GrandparentKey)
		if err != nil {
			return "", 0, err
		}

		id := domain.GUID{
			GUIDS: guid.GUIDS,
			GUID:  guid.GUID,
		}

		return id.PlexAgent(p.Metadata.Type)
	}
	return "", 0, nil
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
