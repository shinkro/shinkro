package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock user service for testing
type mockUserService struct {
	userCount int
	user      *domain.User
	err       error
}

func (m *mockUserService) GetUserCount(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.userCount, nil
}

func (m *mockUserService) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.user != nil && m.user.Username == username {
		return m.user, nil
	}
	return nil, errors.New("user not found")
}

func (m *mockUserService) CreateUser(ctx context.Context, req domain.CreateUserRequest) error {
	if m.err != nil {
		return m.err
	}
	m.userCount++
	return nil
}

func (m *mockUserService) Update(ctx context.Context, req domain.UpdateUserRequest) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func TestService_CreateHash(t *testing.T) {
	service := NewService(zerolog.Nop(), &mockUserService{})

	tests := []struct {
		name          string
		password      string
		expectedError bool
		validate      func(*testing.T, string)
	}{
		{
			name:          "valid password",
			password:      "testpassword123",
			expectedError: false,
			validate: func(t *testing.T, hash string) {
				assert.NotEmpty(t, hash)
				assert.Contains(t, hash, "argon2id") // Argon2 hash format
			},
		},
		{
			name:          "empty password",
			password:      "",
			expectedError: true,
		},
		{
			name:          "long password",
			password:      "verylongpasswordthatshouldworkfine123456789",
			expectedError: false,
			validate: func(t *testing.T, hash string) {
				assert.NotEmpty(t, hash)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := service.CreateHash(tt.password)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, hash)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, hash)
				if tt.validate != nil {
					tt.validate(t, hash)
				}
			}
		})
	}
}

func TestService_ComparePasswordAndHash(t *testing.T) {
	service := NewService(zerolog.Nop(), &mockUserService{})

	// Create a valid hash first
	validPassword := "testpassword123"
	validHash, err := service.CreateHash(validPassword)
	require.NoError(t, err)
	require.NotEmpty(t, validHash)

	tests := []struct {
		name          string
		password      string
		hash          string
		expectedMatch bool
		expectedError bool
	}{
		{
			name:          "correct password",
			password:      validPassword,
			hash:          validHash,
			expectedMatch: true,
			expectedError: false,
		},
		{
			name:          "incorrect password",
			password:      "wrongpassword",
			hash:          validHash,
			expectedMatch: false,
			expectedError: false,
		},
		{
			name:          "invalid hash format",
			password:      validPassword,
			hash:          "invalidhash",
			expectedMatch: false,
			expectedError: true,
		},
		{
			name:          "empty password",
			password:      "",
			hash:          validHash,
			expectedMatch: false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := service.ComparePasswordAndHash(tt.password, tt.hash)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMatch, match)
			}
		})
	}
}

func TestService_Login(t *testing.T) {
	// Create a user with a hashed password
	userService := &mockUserService{}
	authService := NewService(zerolog.Nop(), userService)

	// Create a test user with hashed password
	testPassword := "testpassword123"
	hashedPassword, err := authService.CreateHash(testPassword)
	require.NoError(t, err)

	testUser := testdata.NewMockUser()
	testUser.Password = hashedPassword
	userService.user = testUser

	tests := []struct {
		name          string
		username      string
		password      string
		repoUser      *domain.User
		repoError     error
		expectedError bool
		errContains   string
	}{
		{
			name:          "valid credentials",
			username:      "testuser",
			password:      testPassword,
			repoUser:      testUser,
			expectedError: false,
		},
		{
			name:          "invalid username",
			username:      "wronguser",
			password:      testPassword,
			repoUser:      nil,
			repoError:     errors.New("user not found"),
			expectedError: true,
			errContains:   "invalid login",
		},
		{
			name:          "invalid password",
			username:      "testuser",
			password:      "wrongpassword",
			repoUser:      testUser,
			expectedError: true,
			errContains:   "invalid login",
		},
		{
			name:          "empty username",
			username:      "",
			password:      testPassword,
			expectedError: true,
			errContains:   "empty credentials",
		},
		{
			name:          "empty password",
			username:      "testuser",
			password:      "",
			expectedError: true,
			errContains:   "empty credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userService.user = tt.repoUser
			userService.err = tt.repoError

			result, err := authService.Login(context.Background(), tt.username, tt.password)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.username, result.Username)
			}
		})
	}
}

func TestService_CreateUser(t *testing.T) {
	userService := &mockUserService{}
	authService := NewService(zerolog.Nop(), userService)

	tests := []struct {
		name          string
		req           domain.CreateUserRequest
		repoCount     int
		repoError     error
		expectedError bool
		errContains   string
	}{
		{
			name: "create first user",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "password123",
			},
			repoCount:     0,
			expectedError: false,
		},
		{
			name: "empty username",
			req: domain.CreateUserRequest{
				Username: "",
				Password: "password123",
			},
			expectedError: true,
			errContains:   "empty username",
		},
		{
			name: "empty password",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "",
			},
			expectedError: true,
			errContains:   "empty password",
		},
		{
			name: "cannot create second user",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "password123",
			},
			repoCount:     1,
			expectedError: true,
			errContains:   "only 1 user account is supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userService.userCount = tt.repoCount
			userService.err = tt.repoError

			err := authService.CreateUser(context.Background(), tt.req)
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_UpdateUser(t *testing.T) {
	userService := &mockUserService{}
	authService := NewService(zerolog.Nop(), userService)

	// Create a test user with hashed password
	testPassword := "currentpassword123"
	hashedPassword, err := authService.CreateHash(testPassword)
	require.NoError(t, err)

	testUser := testdata.NewMockUser()
	testUser.Password = hashedPassword
	userService.user = testUser

	tests := []struct {
		name          string
		req           domain.UpdateUserRequest
		repoError     error
		expectedError bool
		errContains   string
	}{
		{
			name: "update username only",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				UsernameNew:     "newusername",
				PasswordCurrent: testPassword,
			},
			expectedError: false,
		},
		{
			name: "update password",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				PasswordCurrent: testPassword,
				PasswordNew:     "newpassword123",
			},
			expectedError: false,
		},
		{
			name: "empty current password",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				PasswordCurrent: "",
			},
			expectedError: true,
			errContains:   "empty current password",
		},
		{
			name: "wrong current password",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				PasswordCurrent: "wrongpassword",
			},
			expectedError: true,
			errContains:   "invalid login",
		},
		{
			name: "new password same as current",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				PasswordCurrent: testPassword,
				PasswordNew:     testPassword,
			},
			expectedError: true,
			errContains:   "new password must be different",
		},
		{
			name: "user not found",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "nonexistent",
				PasswordCurrent: testPassword,
			},
			repoError:     errors.New("user not found"),
			expectedError: true,
			errContains:   "invalid login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userService.err = tt.repoError

			err := authService.UpdateUser(context.Background(), tt.req)
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_ResetPassword(t *testing.T) {
	userService := &mockUserService{}
	authService := NewService(zerolog.Nop(), userService)

	testUser := testdata.NewMockUser()
	userService.user = testUser

	tests := []struct {
		name          string
		username      string
		newPassword   string
		repoError     error
		expectedError bool
		errContains   string
	}{
		{
			name:          "reset password",
			username:      "testuser",
			newPassword:   "newpassword123",
			expectedError: false,
		},
		{
			name:          "empty username",
			username:      "",
			newPassword:   "newpassword123",
			expectedError: true,
			errContains:   "empty username",
		},
		{
			name:          "empty new password",
			username:      "testuser",
			newPassword:   "",
			expectedError: true,
			errContains:   "empty new password",
		},
		{
			name:          "user not found",
			username:      "nonexistent",
			newPassword:   "newpassword123",
			repoError:     errors.New("user not found"),
			expectedError: true,
			errContains:   "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userService.err = tt.repoError

			err := authService.ResetPassword(context.Background(), tt.username, tt.newPassword)
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

