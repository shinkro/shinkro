package user

import (
	"context"
	"errors"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Mock repository for testing
type mockUserRepo struct {
	userCount     int
	user          *domain.User
	err           error
	storeError    error
	getCountError error
}

func (m *mockUserRepo) GetUserCount(ctx context.Context) (int, error) {
	if m.getCountError != nil {
		return 0, m.getCountError
	}
	if m.err != nil {
		return 0, m.err
	}
	return m.userCount, nil
}

func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.user != nil && m.user.Username == username {
		return m.user, nil
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepo) Store(ctx context.Context, req domain.CreateUserRequest) error {
	if m.storeError != nil {
		return m.storeError
	}
	if m.err != nil {
		return m.err
	}
	m.userCount++
	return nil
}

func (m *mockUserRepo) Update(ctx context.Context, req domain.UpdateUserRequest) error {
	if m.err != nil {
		return m.err
	}
	if m.user != nil {
		m.user.Username = req.UsernameNew
	}
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, username string) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func TestService_GetUserCount(t *testing.T) {
	tests := []struct {
		name          string
		repoCount     int
		repoError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "zero users",
			repoCount:     0,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "one user",
			repoCount:     1,
			expectedCount: 1,
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
			repo := &mockUserRepo{
				userCount: tt.repoCount,
				err:       tt.repoError,
			}
			service := NewService(repo, zerolog.Nop())

			result, err := service.GetUserCount(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, result)
			}
		})
	}
}

func TestService_FindByUsername(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		repoUser      *domain.User
		repoError     error
		expectedError bool
		validate      func(*testing.T, *domain.User)
	}{
		{
			name:     "user found",
			username: "testuser",
			repoUser: testdata.NewMockUser(),
			validate: func(t *testing.T, user *domain.User) {
				assert.Equal(t, "testuser", user.Username)
			},
		},
		{
			name:          "user not found",
			username:      "nonexistent",
			repoUser:      nil,
			repoError:     errors.New("user not found"),
			expectedError: true,
		},
		{
			name:          "repository error",
			username:      "testuser",
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				user: tt.repoUser,
				err:  tt.repoError,
			}
			service := NewService(repo, zerolog.Nop())

			result, err := service.FindByUsername(context.Background(), tt.username)
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

func TestService_CreateUser(t *testing.T) {
	tests := []struct {
		name          string
		req           domain.CreateUserRequest
		repoCount     int
		getCountError error
		storeError    error
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
			name: "cannot create second user",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "password123",
			},
			repoCount:     1,
			expectedError: true,
			errContains:   "only 1 user account is supported",
		},
		{
			name: "repository error on count",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "password123",
			},
			repoCount:     0,
			getCountError: errors.New("database error"),
			expectedError: true,
		},
		{
			name: "repository error on store",
			req: domain.CreateUserRequest{
				Username: "newuser",
				Password: "password123",
			},
			repoCount:     0,
			storeError:    errors.New("store error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				userCount:     tt.repoCount,
				getCountError: tt.getCountError,
				storeError:    tt.storeError,
			}
			service := NewService(repo, zerolog.Nop())

			err := service.CreateUser(context.Background(), tt.req)
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

func TestService_Update(t *testing.T) {
	tests := []struct {
		name          string
		req           domain.UpdateUserRequest
		repoError     error
		expectedError bool
	}{
		{
			name: "update user",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
				UsernameNew:     "newusername",
			},
			expectedError: false,
		},
		{
			name: "repository error",
			req: domain.UpdateUserRequest{
				UsernameCurrent: "testuser",
			},
			repoError:     errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				user: testdata.NewMockUser(),
				err:  tt.repoError,
			}
			service := NewService(repo, zerolog.Nop())

			err := service.Update(context.Background(), tt.req)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
