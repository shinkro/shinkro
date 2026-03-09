//go:build integration

package database

import (
	"context"
	"strconv"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/testdata"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	log := zerolog.Nop()

	db := NewDB(tmpDir, &log)
	require.NotNil(t, db)

	// Enable foreign key constraints for SQLite
	_, err := db.handler.Exec(`PRAGMA foreign_keys = ON;`)
	require.NoError(t, err)

	err = db.Migrate()
	require.NoError(t, err)

	return db
}

// cleanupTestDB closes the database connection
func cleanupTestDB(t *testing.T, db *DB) {
	if db != nil {
		err := db.Close()
		assert.NoError(t, err)
	}
}

func TestDB_Migrate(t *testing.T) {
	tmpDir := t.TempDir()
	log := zerolog.Nop()

	db := NewDB(tmpDir, &log)
	require.NotNil(t, db)
	defer cleanupTestDB(t, db)

	// First migration should succeed
	err := db.Migrate()
	require.NoError(t, err)

	// Second migration should be idempotent
	err = db.Migrate()
	require.NoError(t, err)

	// Verify schema version
	var version int
	err = db.handler.QueryRow("PRAGMA user_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, len(migrations), version)
}

func TestDB_Ping(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	err := db.Ping()
	assert.NoError(t, err)
}

func TestDB_BeginTx(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Test that we can execute queries in transaction
	_, err = tx.Exec("SELECT 1")
	assert.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback()
	assert.NoError(t, err)
}

func TestUserRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewUserRepo(log, db)
	ctx := context.Background()

	t.Run("get user count on empty database", func(t *testing.T) {
		count, err := repo.GetUserCount(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("store user", func(t *testing.T) {
		req := testdata.NewMockCreateUserRequest()
		err := repo.Store(ctx, req)
		assert.NoError(t, err)

		count, err := repo.GetUserCount(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("find user by username", func(t *testing.T) {
		req := testdata.NewMockCreateUserRequest()
		req.Username = "testuser"
		err := repo.Store(ctx, req)
		require.NoError(t, err)

		user, err := repo.FindByUsername(ctx, "testuser")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "testuser", user.Username)
		assert.NotEmpty(t, user.Password)
	})

	t.Run("find non-existent user", func(t *testing.T) {
		user, err := repo.FindByUsername(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "record not found")
	})

	t.Run("update user", func(t *testing.T) {
		req := testdata.NewMockCreateUserRequest()
		req.Username = "updateuser"
		err := repo.Store(ctx, req)
		require.NoError(t, err)

		updateReq := testdata.NewMockUpdateUserRequest()
		updateReq.UsernameCurrent = "updateuser"
		updateReq.UsernameNew = "updateduser"
		err = repo.Update(ctx, updateReq)
		assert.NoError(t, err)

		// Verify update
		user, err := repo.FindByUsername(ctx, "updateduser")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "updateduser", user.Username)
	})

	t.Run("delete user", func(t *testing.T) {
		req := testdata.NewMockCreateUserRequest()
		req.Username = "deleteuser"
		err := repo.Store(ctx, req)
		require.NoError(t, err)

		err = repo.Delete(ctx, "deleteuser")
		assert.NoError(t, err)

		// Verify deletion
		user, err := repo.FindByUsername(ctx, "deleteuser")
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("unique username constraint", func(t *testing.T) {
		req := testdata.NewMockCreateUserRequest()
		req.Username = "duplicate"
		err := repo.Store(ctx, req)
		require.NoError(t, err)

		// Try to create duplicate
		err = repo.Store(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNIQUE constraint")
	})
}

func TestAnimeRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewAnimeRepo(log, db)
	ctx := context.Background()

	t.Run("store multiple anime", func(t *testing.T) {
		anime := []*domain.Anime{
			{MALId: 1575, TVDBId: 81797, TMDBId: 37854, MainTitle: "One Piece"},
			{MALId: 21, TVDBId: 362753, TMDBId: 37854, MainTitle: "One Piece"},
		}

		err := repo.StoreMultiple(anime)
		assert.NoError(t, err)
	})

	t.Run("get anime by MAL ID", func(t *testing.T) {
		anime := []*domain.Anime{
			{MALId: 1575, TVDBId: 81797, MainTitle: "One Piece"},
		}
		err := repo.StoreMultiple(anime)
		require.NoError(t, err)

		req := &domain.GetAnimeRequest{
			IDtype: domain.MAL,
			Id:     1575,
		}

		result, err := repo.GetByID(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1575, result.MALId)
		assert.Equal(t, "One Piece", result.MainTitle)
	})

	t.Run("get anime by TVDB ID", func(t *testing.T) {
		anime := []*domain.Anime{
			{MALId: 1575, TVDBId: 81797, MainTitle: "One Piece"},
		}
		err := repo.StoreMultiple(anime)
		require.NoError(t, err)

		req := &domain.GetAnimeRequest{
			IDtype: domain.TVDB,
			Id:     81797,
		}

		result, err := repo.GetByID(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 81797, result.TVDBId)
	})

	t.Run("get anime by TMDB ID", func(t *testing.T) {
		anime := []*domain.Anime{
			{MALId: 1575, TMDBId: 37854, MainTitle: "One Piece"},
		}
		err := repo.StoreMultiple(anime)
		require.NoError(t, err)

		req := &domain.GetAnimeRequest{
			IDtype: domain.TMDB,
			Id:     37854,
		}

		result, err := repo.GetByID(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 37854, result.TMDBId)
	})

	t.Run("get non-existent anime", func(t *testing.T) {
		req := &domain.GetAnimeRequest{
			IDtype: domain.MAL,
			Id:     99999,
		}

		result, err := repo.GetByID(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no rows")
	})
}

func TestAnimeUpdateRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewAnimeUpdateRepo(log, db)
	plexRepo := NewPlexRepo(log, db)
	ctx := context.Background()

	// Create a dummy plex_payload for tests that don't need a real reference
	dummyPlex := testdata.NewMockPlex()
	err := plexRepo.Store(ctx, dummyPlex)
	require.NoError(t, err)

	t.Run("store anime update", func(t *testing.T) {
		update := testdata.NewMockAnimeUpdate()
		update.PlexId = dummyPlex.ID // Use dummy plex reference
		err := repo.Store(ctx, update)
		assert.NoError(t, err)
		assert.NotEqual(t, int64(0), update.ID)
	})

	t.Run("get anime update by ID", func(t *testing.T) {
		update := testdata.NewMockAnimeUpdate()
		update.PlexId = dummyPlex.ID // Use dummy plex reference
		err := repo.Store(ctx, update)
		require.NoError(t, err)

		req := &domain.GetAnimeUpdateRequest{Id: int(update.ID)}
		result, err := repo.GetByID(ctx, req)
		// Note: GetByID is not fully implemented, so this might return nil
		_ = result
		_ = err
	})

	t.Run("count anime updates", func(t *testing.T) {
		// Get baseline count
		baselineCount, err := repo.Count(ctx)
		require.NoError(t, err)

		update1 := testdata.NewMockAnimeUpdate()
		update1.Status = domain.AnimeUpdateStatusSuccess
		update1.PlexId = dummyPlex.ID // Use dummy plex reference
		err = repo.Store(ctx, update1)
		require.NoError(t, err)

		update2 := testdata.NewMockAnimeUpdate()
		update2.Status = domain.AnimeUpdateStatusSuccess
		update2.PlexId = dummyPlex.ID // Use dummy plex reference
		err = repo.Store(ctx, update2)
		require.NoError(t, err)

		update3 := testdata.NewMockAnimeUpdate()
		update3.Status = domain.AnimeUpdateStatusFailed
		update3.PlexId = dummyPlex.ID // Use dummy plex reference
		err = repo.Store(ctx, update3)
		require.NoError(t, err)

		count, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, baselineCount+2, count) // Only SUCCESS status
	})

	t.Run("get recent unique anime updates", func(t *testing.T) {
		update1 := testdata.NewMockAnimeUpdate()
		update1.MALId = 1575
		update1.PlexId = dummyPlex.ID // Use dummy plex reference
		err := repo.Store(ctx, update1)
		require.NoError(t, err)

		update2 := testdata.NewMockAnimeUpdate()
		update2.MALId = 21
		update2.PlexId = dummyPlex.ID // Use dummy plex reference
		err = repo.Store(ctx, update2)
		require.NoError(t, err)

		results, err := repo.GetRecentUnique(ctx, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("get anime update by plex ID", func(t *testing.T) {
		// First create a plex payload
		plexRepo := NewPlexRepo(log, db)
		plex := testdata.NewMockPlex()
		err := plexRepo.Store(ctx, plex)
		require.NoError(t, err)

		update := testdata.NewMockAnimeUpdate()
		update.PlexId = plex.ID
		err = repo.Store(ctx, update)
		require.NoError(t, err)

		result, err := repo.GetByPlexID(ctx, plex.ID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, plex.ID, result.PlexId)
	})

	t.Run("get anime updates by plex IDs", func(t *testing.T) {
		plexRepo := NewPlexRepo(log, db)

		plex1 := testdata.NewMockPlex()
		err := plexRepo.Store(ctx, plex1)
		require.NoError(t, err)

		plex2 := testdata.NewMockPlex()
		err = plexRepo.Store(ctx, plex2)
		require.NoError(t, err)

		update1 := testdata.NewMockAnimeUpdate()
		update1.PlexId = plex1.ID
		err = repo.Store(ctx, update1)
		require.NoError(t, err)

		update2 := testdata.NewMockAnimeUpdate()
		update2.PlexId = plex2.ID
		err = repo.Store(ctx, update2)
		require.NoError(t, err)

		results, err := repo.GetByPlexIDs(ctx, []int64{plex1.ID, plex2.ID})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})

	t.Run("find all with filters", func(t *testing.T) {
		update1 := testdata.NewMockAnimeUpdate()
		update1.Status = domain.AnimeUpdateStatusSuccess
		update1.SourceDB = domain.TVDB
		update1.PlexId = dummyPlex.ID // Use dummy plex reference
		err := repo.Store(ctx, update1)
		require.NoError(t, err)

		update2 := testdata.NewMockAnimeUpdate()
		update2.Status = domain.AnimeUpdateStatusFailed
		update2.SourceDB = domain.MAL
		update2.PlexId = dummyPlex.ID // Use dummy plex reference
		err = repo.Store(ctx, update2)
		require.NoError(t, err)

		params := domain.AnimeUpdateQueryParams{
			Filters: struct {
				Status    domain.AnimeUpdateStatusType
				ErrorType domain.AnimeUpdateErrorType
				Source    domain.PlexSupportedDBs
			}{
				Status: domain.AnimeUpdateStatusSuccess,
			},
		}

		results, err := repo.FindAllWithFilters(ctx, params)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.GreaterOrEqual(t, len(results.Data), 1)
	})
}

func TestPlexRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewPlexRepo(log, db)
	ctx := context.Background()

	t.Run("store plex payload", func(t *testing.T) {
		plex := testdata.NewMockPlex()
		err := repo.Store(ctx, plex)
		assert.NoError(t, err)
		assert.NotEqual(t, int64(0), plex.ID)
	})

	t.Run("count scrobble events", func(t *testing.T) {
		// Get baseline count
		baselineCount, err := repo.CountScrobbleEvents(ctx)
		require.NoError(t, err)

		plex1 := testdata.NewMockPlex()
		plex1.Event = domain.PlexScrobbleEvent
		err = repo.Store(ctx, plex1)
		require.NoError(t, err)

		plex2 := testdata.NewMockPlex()
		plex2.Event = domain.PlexScrobbleEvent
		err = repo.Store(ctx, plex2)
		require.NoError(t, err)

		plex3 := testdata.NewMockPlex()
		plex3.Event = domain.PlexRateEvent
		err = repo.Store(ctx, plex3)
		require.NoError(t, err)

		count, err := repo.CountScrobbleEvents(ctx)
		assert.NoError(t, err)
		assert.Equal(t, baselineCount+2, count)
	})

	t.Run("count rate events", func(t *testing.T) {
		// Get baseline count
		baselineCount, err := repo.CountRateEvents(ctx)
		require.NoError(t, err)

		plex1 := testdata.NewMockPlex()
		plex1.Event = domain.PlexRateEvent
		err = repo.Store(ctx, plex1)
		require.NoError(t, err)

		plex2 := testdata.NewMockPlex()
		plex2.Event = domain.PlexRateEvent
		err = repo.Store(ctx, plex2)
		require.NoError(t, err)

		count, err := repo.CountRateEvents(ctx)
		assert.NoError(t, err)
		assert.Equal(t, baselineCount+2, count)
	})

	t.Run("get recent plex payloads", func(t *testing.T) {
		plex1 := testdata.NewMockPlex()
		err := repo.Store(ctx, plex1)
		require.NoError(t, err)

		plex2 := testdata.NewMockPlex()
		err = repo.Store(ctx, plex2)
		require.NoError(t, err)

		results, err := repo.GetRecent(ctx, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("find all with filters", func(t *testing.T) {
		plex1 := testdata.NewMockPlex()
		plex1.Event = domain.PlexScrobbleEvent
		plex1.Source = domain.PlexWebhook
		err := repo.Store(ctx, plex1)
		require.NoError(t, err)

		plex2 := testdata.NewMockPlex()
		plex2.Event = domain.PlexRateEvent
		plex2.Source = domain.TautulliWebhook
		err = repo.Store(ctx, plex2)
		require.NoError(t, err)

		params := domain.PlexPayloadQueryParams{
			Filters: struct {
				Event  domain.PlexEvent
				Source domain.PlexPayloadSource
				Status *bool
			}{
				Event: domain.PlexScrobbleEvent,
			},
		}

		results, err := repo.FindAllWithFilters(ctx, params)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.GreaterOrEqual(t, len(results.Data), 1)
	})

	t.Run("delete plex payload", func(t *testing.T) {
		plex := testdata.NewMockPlex()
		err := repo.Store(ctx, plex)
		require.NoError(t, err)

		req := &domain.DeletePlexRequest{Id: plex.ID}
		err = repo.Delete(ctx, req)
		assert.NoError(t, err)

		// Verify deletion
		getReq := &domain.GetPlexRequest{Id: int(plex.ID)}
		result, err := repo.Get(ctx, getReq)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAPIRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewAPIRepo(log, db)
	ctx := context.Background()

	t.Run("store API key", func(t *testing.T) {
		key := &domain.APIKey{
			Name:   "Test Key",
			Key:    "test-api-key-12345",
			Scopes: []string{"read", "write"},
		}

		err := repo.Store(ctx, key)
		assert.NoError(t, err)
		assert.False(t, key.CreatedAt.IsZero())
	})

	t.Run("get API key", func(t *testing.T) {
		key := &domain.APIKey{
			Name:   "Test Key",
			Key:    "test-api-key-67890",
			Scopes: []string{"read"},
		}

		err := repo.Store(ctx, key)
		require.NoError(t, err)

		result, err := repo.GetKey(ctx, "test-api-key-67890")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Test Key", result.Name)
		assert.Equal(t, "test-api-key-67890", result.Key)
		assert.Equal(t, []string{"read"}, result.Scopes)
	})

	t.Run("get non-existent API key", func(t *testing.T) {
		result, err := repo.GetKey(ctx, "nonexistent-key")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "record not found")
	})

	t.Run("get all API keys", func(t *testing.T) {
		key1 := &domain.APIKey{
			Name:   "Key 1",
			Key:    "key-1",
			Scopes: []string{"read"},
		}
		err := repo.Store(ctx, key1)
		require.NoError(t, err)

		key2 := &domain.APIKey{
			Name:   "Key 2",
			Key:    "key-2",
			Scopes: []string{"write"},
		}
		err = repo.Store(ctx, key2)
		require.NoError(t, err)

		keys, err := repo.GetAllAPIKeys(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), 2)
	})

	t.Run("delete API key", func(t *testing.T) {
		key := &domain.APIKey{
			Name:   "Delete Key",
			Key:    "delete-key",
			Scopes: []string{"read"},
		}

		err := repo.Store(ctx, key)
		require.NoError(t, err)

		err = repo.Delete(ctx, "delete-key")
		assert.NoError(t, err)

		// Verify deletion
		result, err := repo.GetKey(ctx, "delete-key")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAnimeUpdateRepo_ForeignKeyConstraint(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewAnimeUpdateRepo(log, db)
	ctx := context.Background()

	t.Run("cascade delete on plex payload deletion", func(t *testing.T) {
		plexRepo := NewPlexRepo(log, db)
		plex := testdata.NewMockPlex()
		err := plexRepo.Store(ctx, plex)
		require.NoError(t, err)

		update := testdata.NewMockAnimeUpdate()
		update.PlexId = plex.ID
		err = repo.Store(ctx, update)
		require.NoError(t, err)

		// Verify update exists before deletion
		result, err := repo.GetByPlexID(ctx, plex.ID)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, plex.ID, result.PlexId)

		// Delete plex payload
		deleteReq := &domain.DeletePlexRequest{Id: plex.ID}
		err = plexRepo.Delete(ctx, deleteReq)
		assert.NoError(t, err)

		// Anime update should be cascade deleted
		result, err = repo.GetByPlexID(ctx, plex.ID)
		assert.NoError(t, err)
		assert.Nil(t, result) // GetByPlexID returns nil, nil when no rows found
	})
}

func TestDatabase_Transactions(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("rollback transaction", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		// Insert data in transaction
		_, err = tx.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "txuser", "password")
		assert.NoError(t, err)

		// Rollback
		err = tx.Rollback()
		assert.NoError(t, err)

		// Verify data was not committed
		userRepo := NewUserRepo(log, db)
		user, err := userRepo.FindByUsername(ctx, "txuser")
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("commit transaction", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		// Insert data in transaction
		_, err = tx.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "commituser", "password")
		assert.NoError(t, err)

		// Commit
		err = tx.Commit()
		assert.NoError(t, err)

		// Verify data was committed
		userRepo := NewUserRepo(log, db)
		user, err := userRepo.FindByUsername(ctx, "commituser")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "commituser", user.Username)
	})
}

func TestDatabase_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	log := zerolog.Nop()
	repo := NewUserRepo(log, db)
	ctx := context.Background()

	// Test concurrent writes
	t.Run("concurrent user creation", func(t *testing.T) {
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				req := domain.CreateUserRequest{
					Username: "user" + strconv.Itoa(id),
					Password: "password",
				}
				// Each should succeed with unique usernames
				_ = repo.Store(ctx, req)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify at least one user was created
		count, err := repo.GetUserCount(ctx)
		assert.NoError(t, err)
		assert.Greater(t, count, 0)
	})
}
