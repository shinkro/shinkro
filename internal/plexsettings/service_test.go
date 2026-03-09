package plexsettings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock repository for testing
type mockPlexSettingsRepo struct {
	settings *domain.PlexSettings
	err      error
}

func (m *mockPlexSettingsRepo) Store(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.settings = &ps
	return &ps, nil
}

func (m *mockPlexSettingsRepo) Get(ctx context.Context) (*domain.PlexSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.settings == nil {
		return testdata.NewMockPlexSettings(), nil
	}
	return m.settings, nil
}

func (m *mockPlexSettingsRepo) Update(ctx context.Context, ps domain.PlexSettings) (*domain.PlexSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.settings = &ps
	return &ps, nil
}

func (m *mockPlexSettingsRepo) Delete(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

// Helper to generate a valid 32-byte hex key
func generateTestKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}

// Helper to generate a valid 12-byte IV for AES-GCM
func generateTestIV() []byte {
	iv := make([]byte, 12)
	rand.Read(iv)
	return iv
}

func TestService_EncryptDecrypt(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockPlexSettingsRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	tests := []struct {
		name          string
		plaintext     []byte
		iv            []byte
		expectedError bool
	}{
		{
			name:          "encrypt and decrypt token",
			plaintext:     []byte("test-token-12345"),
			iv:            generateTestIV(),
			expectedError: false,
		},
		{
			name:          "encrypt and decrypt empty string",
			plaintext:     []byte(""),
			iv:            generateTestIV(),
			expectedError: false,
		},
		{
			name:          "encrypt and decrypt long string",
			plaintext:     []byte("this is a very long token string that should be encrypted and decrypted correctly"),
			iv:            generateTestIV(),
			expectedError: false,
		},
		{
			name:          "encrypt and decrypt binary data",
			plaintext:     []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			iv:            generateTestIV(),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := service.encrypt(tt.plaintext, tt.iv)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			assert.NotEqual(t, tt.plaintext, ciphertext) // Should be different

			// Decrypt
			decrypted, err := service.decrypt(ciphertext, tt.iv)
			require.NoError(t, err)
			// Handle empty slice vs nil comparison
			if len(tt.plaintext) == 0 {
				assert.Empty(t, decrypted)
			} else {
				assert.Equal(t, tt.plaintext, decrypted)
			}
		})
	}
}

func TestService_GetEncryptionKey(t *testing.T) {
	tests := []struct {
		name          string
		encryptionKey string
		expectedError bool
		errContains   string
	}{
		{
			name:          "valid 32-byte hex key",
			encryptionKey: generateTestKey(),
			expectedError: false,
		},
		{
			name:          "invalid hex string",
			encryptionKey: "not-a-hex-string",
			expectedError: true,
			errContains:   "invalid hex encryption key",
		},
		{
			name:          "key too short",
			encryptionKey: hex.EncodeToString([]byte("short")),
			expectedError: true,
			errContains:   "encryption key must be 32 bytes",
		},
		{
			name:          "key too long",
			encryptionKey: hex.EncodeToString(make([]byte, 64)),
			expectedError: true,
			errContains:   "encryption key must be 32 bytes",
		},
		{
			name:          "empty key",
			encryptionKey: "",
			expectedError: true,
			errContains:   "encryption key must be 32 bytes", // Empty string decodes to 0 bytes, fails length check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &domain.Config{
				EncryptionKey: tt.encryptionKey,
			}
			service := NewService(config, zerolog.Nop(), &mockPlexSettingsRepo{}).(*service)

			key, err := service.getEncryptionKey()
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, key)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, key)
				assert.Equal(t, 32, len(key))
			}
		})
	}
}

func TestService_Store(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockPlexSettingsRepo{}
	service := NewService(config, zerolog.Nop(), repo)

	tests := []struct {
		name          string
		settings      domain.PlexSettings
		repoError     error
		expectedError bool
		validate      func(*testing.T, *domain.PlexSettings)
	}{
		{
			name: "store with encryption",
			settings: func() domain.PlexSettings {
				ps := *testdata.NewMockPlexSettings()
				ps.Token = []byte("plaintext-token")
				ps.TokenIV = generateTestIV()
				return ps
			}(),
			expectedError: false,
			validate: func(t *testing.T, stored *domain.PlexSettings) {
				assert.NotNil(t, stored)
				assert.NotEqual(t, []byte("plaintext-token"), stored.Token) // Should be encrypted
				assert.NotEmpty(t, stored.Token)
			},
		},
		{
			name: "repository error",
			settings: func() domain.PlexSettings {
				ps := *testdata.NewMockPlexSettings()
				ps.Token = []byte("test-token")
				ps.TokenIV = generateTestIV()
				return ps
			}(),
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoError
			result, err := service.Store(context.Background(), tt.settings)
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

func TestService_GetClient(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockPlexSettingsRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	// Create encrypted token for test
	testToken := []byte("test-plex-token")
	testIV := generateTestIV()
	encryptedToken, err := service.encrypt(testToken, testIV)
	require.NoError(t, err)

	tests := []struct {
		name          string
		settings      *domain.PlexSettings
		expectedError bool
		errContains   string
	}{
		{
			name: "get client with token and IV",
			settings: &domain.PlexSettings{
				Host:     "192.168.1.100",
				Port:     32400,
				TLS:      false,
				TLSSkip:  false,
				Token:    encryptedToken,
				TokenIV:  testIV,
				ClientID: "test-client-id",
			},
			expectedError: false,
		},
		{
			name: "get client with TLS",
			settings: &domain.PlexSettings{
				Host:     "plex.example.com",
				Port:     32400,
				TLS:      true,
				TLSSkip:  false,
				Token:    encryptedToken,
				TokenIV:  testIV,
				ClientID: "test-client-id",
			},
			expectedError: false,
		},
		{
			name: "empty token",
			settings: &domain.PlexSettings{
				Host:    "192.168.1.100",
				Port:    32400,
				Token:   []byte{},
				TokenIV: testIV,
			},
			expectedError: true,
			errContains:   "token or tokenIV is empty",
		},
		{
			name: "empty tokenIV",
			settings: &domain.PlexSettings{
				Host:    "192.168.1.100",
				Port:    32400,
				Token:   encryptedToken,
				TokenIV: []byte{}, // Empty slice is checked before decrypt is called
			},
			expectedError: true,
			errContains:   "token or tokenIV is empty",
		},
		{
			name: "load token from database when missing",
			settings: &domain.PlexSettings{
				Host:    "192.168.1.100",
				Port:    32400,
				Token:   nil,
				TokenIV: nil,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up repo with encrypted token if needed
			if tt.name == "load token from database when missing" {
				repo.settings = &domain.PlexSettings{
					Token:   encryptedToken,
					TokenIV: testIV,
				}
			} else if tt.name == "empty tokenIV" {
				// Set up repo to return empty TokenIV so the check fails
				repo.settings = &domain.PlexSettings{
					Token:   encryptedToken,
					TokenIV: []byte{}, // Empty so check fails
				}
			}

			client, err := service.GetClient(context.Background(), tt.settings)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
