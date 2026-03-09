package api

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock repository for testing
type mockAPIRepo struct {
	keys  map[string]*domain.APIKey
	err   error
	storeErr error
}

func (m *mockAPIRepo) GetAllAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	keys := make([]domain.APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, *key)
	}
	return keys, nil
}

func (m *mockAPIRepo) GetKey(ctx context.Context, key string) (*domain.APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	if apiKey, ok := m.keys[key]; ok {
		return apiKey, nil
	}
	return nil, errors.New("key not found")
}

func (m *mockAPIRepo) Store(ctx context.Context, key *domain.APIKey) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	if m.keys == nil {
		m.keys = make(map[string]*domain.APIKey)
	}
	m.keys[key.Key] = key
	return nil
}

func (m *mockAPIRepo) Delete(ctx context.Context, key string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.keys[key]; !ok {
		return errors.New("key not found")
	}
	delete(m.keys, key)
	return nil
}

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name   string
		length int
		validate func(*testing.T, string)
	}{
		{
			name:   "generate 16 byte token",
			length: 16,
			validate: func(t *testing.T, token string) {
				assert.NotEmpty(t, token)
				// Hex encoding doubles the length
				assert.Equal(t, 32, len(token))
				// Should be valid hex
				matched, _ := regexp.MatchString("^[0-9a-f]{32}$", token)
				assert.True(t, matched, "token should be valid hex")
			},
		},
		{
			name:   "generate 32 byte token",
			length: 32,
			validate: func(t *testing.T, token string) {
				assert.NotEmpty(t, token)
				// Hex encoding doubles the length
				assert.Equal(t, 64, len(token))
				// Should be valid hex
				matched, _ := regexp.MatchString("^[0-9a-f]{64}$", token)
				assert.True(t, matched, "token should be valid hex")
			},
		},
		{
			name:   "generate 8 byte token",
			length: 8,
			validate: func(t *testing.T, token string) {
				assert.NotEmpty(t, token)
				assert.Equal(t, 16, len(token))
			},
		},
		{
			name:   "tokens should be unique",
			length: 16,
			validate: func(t *testing.T, token string) {
				// Generate multiple tokens and verify they're different
				tokens := make(map[string]bool)
				for i := 0; i < 10; i++ {
					newToken := GenerateSecureToken(16)
					assert.False(t, tokens[newToken], "tokens should be unique")
					tokens[newToken] = true
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := GenerateSecureToken(tt.length)
			if tt.validate != nil {
				tt.validate(t, token)
			}
		})
	}
}

func TestService_Store(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo)

	tests := []struct {
		name          string
		apiKey        *domain.APIKey
		storeErr      error
		expectedError bool
		validate      func(*testing.T, *domain.APIKey)
	}{
		{
			name: "store new API key",
			apiKey: &domain.APIKey{
				Name: "Test Key",
			},
			expectedError: false,
			validate: func(t *testing.T, key *domain.APIKey) {
				assert.NotEmpty(t, key.Key)
				assert.Equal(t, "Test Key", key.Name)
				// Key should be 32 hex characters (16 bytes)
				assert.Equal(t, 32, len(key.Key))
			},
		},
		{
			name: "repository error",
			apiKey: &domain.APIKey{
				Name: "Test Key",
			},
			storeErr:      errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.storeErr = tt.storeErr
			err := service.Store(context.Background(), tt.apiKey)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tt.apiKey)
				}
			}
		})
	}
}

func TestService_ValidateAPIKey(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo)

	// Add a key to the repository
	testKey := &domain.APIKey{
		Key:  "test-api-key-12345",
		Name: "Test Key",
	}
	repo.keys[testKey.Key] = testKey

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "valid key",
			key:      "test-api-key-12345",
			expected: true,
		},
		{
			name:     "invalid key",
			key:      "invalid-key",
			expected: false,
		},
		{
			name:     "empty key",
			key:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.ValidateAPIKey(context.Background(), tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_ValidateAPIKey_Cache(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo).(*service)

	testKey := &domain.APIKey{
		Key:  "cached-key-12345",
		Name: "Cached Key",
	}
	repo.keys[testKey.Key] = testKey

	// First call should hit the repository
	result1 := service.ValidateAPIKey(context.Background(), testKey.Key)
	assert.True(t, result1)
	assert.Equal(t, 1, len(service.keyCache))

	// Second call should hit the cache
	result2 := service.ValidateAPIKey(context.Background(), testKey.Key)
	assert.True(t, result2)
	assert.Equal(t, 1, len(service.keyCache))
}

func TestService_List(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo)

	tests := []struct {
		name          string
		setupKeys     []*domain.APIKey
		repoError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "empty list",
			setupKeys:     []*domain.APIKey{},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "list with keys",
			setupKeys: []*domain.APIKey{
				{Key: "key1", Name: "Key 1"},
				{Key: "key2", Name: "Key 2"},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "repository error",
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.keys = make(map[string]*domain.APIKey)
			for _, key := range tt.setupKeys {
				repo.keys[key.Key] = key
			}
			repo.err = tt.repoError

			result, err := service.List(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(result))
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo).(*service)

	testKey := &domain.APIKey{
		Key:  "key-to-delete",
		Name: "Delete Me",
	}
	repo.keys[testKey.Key] = testKey
	service.keyCache[testKey.Key] = *testKey

	tests := []struct {
		name          string
		key           string
		repoError     error
		expectedError bool
		validate      func(*testing.T)
	}{
		{
			name:          "delete existing key",
			key:           "key-to-delete",
			expectedError: false,
			validate: func(t *testing.T) {
				// Key should be removed from cache
				_, exists := service.keyCache["key-to-delete"]
				assert.False(t, exists)
			},
		},
		{
			name:          "delete non-existent key",
			key:           "non-existent",
			expectedError: true,
		},
		{
			name:          "repository error",
			key:           "key-to-delete",
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.err = tt.repoError
			err := service.Delete(context.Background(), tt.key)
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

func TestService_List_WithCache(t *testing.T) {
	repo := &mockAPIRepo{keys: make(map[string]*domain.APIKey)}
	service := NewService(zerolog.Nop(), repo).(*service)

	// Populate cache
	cachedKey := domain.APIKey{Key: "cached-key", Name: "Cached"}
	service.keyCache["cached-key"] = cachedKey

	// List should return from cache
	result, err := service.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "cached-key", result[0].Key)
}

