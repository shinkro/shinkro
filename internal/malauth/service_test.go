package malauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// Mock repository for testing
type mockMalAuthRepo struct {
	malAuth *domain.MalAuth
	err     error
}

func (m *mockMalAuthRepo) Store(ctx context.Context, ma *domain.MalAuth) error {
	if m.err != nil {
		return m.err
	}
	m.malAuth = ma
	return nil
}

func (m *mockMalAuthRepo) Get(ctx context.Context) (*domain.MalAuth, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.malAuth, nil
}

func (m *mockMalAuthRepo) Delete(ctx context.Context) error {
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
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	tests := []struct {
		name          string
		plaintext     []byte
		iv            []byte
		expectedError bool
	}{
		{
			name:          "encrypt and decrypt access token",
			plaintext:     []byte(`{"access_token":"test-token","token_type":"Bearer"}`),
			iv:            generateTestIV(),
			expectedError: false,
		},
		{
			name:          "encrypt and decrypt client ID",
			plaintext:     []byte("test-client-id-12345"),
			iv:            generateTestIV(),
			expectedError: false,
		},
		{
			name:          "encrypt and decrypt client secret",
			plaintext:     []byte("test-client-secret-67890"),
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
			plaintext:     []byte("this is a very long client secret that should be encrypted and decrypted correctly"),
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
			service := NewService(config, zerolog.Nop(), &mockMalAuthRepo{}).(*service)

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
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo)

	testIV := generateTestIV()
	testToken := []byte(`{"access_token":"test","token_type":"Bearer"}`)

	tests := []struct {
		name          string
		malAuth       *domain.MalAuth
		repoError     error
		expectedError bool
		validate      func(*testing.T, *domain.MalAuth)
	}{
		{
			name: "store with encryption",
			malAuth: &domain.MalAuth{
				AccessToken: testToken,
				TokenIV:     testIV,
				Config: oauth2.Config{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			expectedError: false,
			validate: func(t *testing.T, stored *domain.MalAuth) {
				assert.NotNil(t, stored)
				// AccessToken, ClientID, and ClientSecret should be encrypted
				assert.NotEqual(t, testToken, stored.AccessToken)
				assert.NotEqual(t, "test-client-id", stored.Config.ClientID)
				assert.NotEqual(t, "test-client-secret", stored.Config.ClientSecret)
				assert.NotEmpty(t, stored.AccessToken)
				assert.NotEmpty(t, stored.Config.ClientID)
				assert.NotEmpty(t, stored.Config.ClientSecret)
			},
		},
		{
			name: "repository error",
			malAuth: &domain.MalAuth{
				AccessToken: testToken,
				TokenIV:     testIV,
				Config: oauth2.Config{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoError
			err := service.Store(context.Background(), tt.malAuth)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tt.malAuth)
				}
			}
		})
	}
}

func TestService_GetDecrypted(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	// Create encrypted credentials
	testIV := generateTestIV()
	testClientID := []byte("test-client-id")
	testClientSecret := []byte("test-client-secret")

	encryptedClientID, err := service.encrypt(testClientID, testIV)
	require.NoError(t, err)
	encryptedClientSecret, err := service.encrypt(testClientSecret, testIV)
	require.NoError(t, err)

	tests := []struct {
		name          string
		malAuth       *domain.MalAuth
		repoError     error
		expectedError bool
		validate      func(*testing.T, *domain.MalAuth)
	}{
		{
			name: "get and decrypt credentials",
			malAuth: &domain.MalAuth{
				AccessToken: []byte("encrypted-token"),
				TokenIV:     testIV,
				Config: oauth2.Config{
					ClientID:     string(encryptedClientID),
					ClientSecret: string(encryptedClientSecret),
				},
			},
			expectedError: false,
			validate: func(t *testing.T, decrypted *domain.MalAuth) {
				assert.Equal(t, "test-client-id", decrypted.Config.ClientID)
				assert.Equal(t, "test-client-secret", decrypted.Config.ClientSecret)
			},
		},
		{
			name:          "repository error",
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.malAuth = tt.malAuth
			repo.err = tt.repoError

			result, err := service.GetDecrypted(context.Background())
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

func TestService_GetDecrypted_DecryptionError(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	testIV := generateTestIV()

	// Create invalid encrypted data (wrong IV)
	testClientID := []byte("test-client-id")
	encryptedClientID, err := service.encrypt(testClientID, generateTestIV()) // Different IV
	require.NoError(t, err)

	repo.malAuth = &domain.MalAuth{
		TokenIV: testIV, // Different IV than used for encryption
		Config: oauth2.Config{
			ClientID:     string(encryptedClientID),
			ClientSecret: "encrypted-secret",
		},
	}

	result, err := service.GetDecrypted(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_Store_EncryptsAllFields(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo)

	testIV := generateTestIV()
	testToken := []byte(`{"access_token":"token","token_type":"Bearer"}`)
	originalClientID := "original-client-id"
	originalClientSecret := "original-client-secret"

	malAuth := &domain.MalAuth{
		AccessToken: testToken,
		TokenIV:     testIV,
		Config: oauth2.Config{
			ClientID:     originalClientID,
			ClientSecret: originalClientSecret,
		},
	}

	err := service.Store(context.Background(), malAuth)
	require.NoError(t, err)

	// Verify all fields are encrypted
	assert.NotEqual(t, testToken, malAuth.AccessToken)
	assert.NotEqual(t, originalClientID, malAuth.Config.ClientID)
	assert.NotEqual(t, originalClientSecret, malAuth.Config.ClientSecret)

	// Verify we can decrypt them back
	decrypted, err := service.GetDecrypted(context.Background())
	require.NoError(t, err)
	assert.Equal(t, originalClientID, decrypted.Config.ClientID)
	assert.Equal(t, originalClientSecret, decrypted.Config.ClientSecret)
}

func TestService_GetMalClient_WithValidToken(t *testing.T) {
	config := &domain.Config{
		EncryptionKey: generateTestKey(),
	}
	repo := &mockMalAuthRepo{}
	service := NewService(config, zerolog.Nop(), repo).(*service)

	testIV := generateTestIV()

	// Create a valid OAuth2 token
	validToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour), // Valid for 1 hour
	}

	tokenJSON, err := json.Marshal(validToken)
	require.NoError(t, err)

	encryptedToken, err := service.encrypt(tokenJSON, testIV)
	require.NoError(t, err)

	testClientID := []byte("test-client-id")
	testClientSecret := []byte("test-client-secret")

	encryptedClientID, err := service.encrypt(testClientID, testIV)
	require.NoError(t, err)
	encryptedClientSecret, err := service.encrypt(testClientSecret, testIV)
	require.NoError(t, err)

	repo.malAuth = &domain.MalAuth{
		AccessToken: encryptedToken,
		TokenIV:     testIV,
		Config: oauth2.Config{
			ClientID:     string(encryptedClientID),
			ClientSecret: string(encryptedClientSecret),
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://myanimelist.net/v1/oauth2/authorize",
				TokenURL:  "https://myanimelist.net/v1/oauth2/token",
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
	}

	// GetMalClient should succeed with valid token and proper OAuth2 config
	client, err := service.GetMalClient(context.Background())
	// The client should be created successfully
	assert.NoError(t, err)
	assert.NotNil(t, client)
	// Verify it's a valid MAL client (has the expected structure)
	assert.NotNil(t, client)
}
