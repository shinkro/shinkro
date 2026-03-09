package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinkro/shinkro/internal/domain"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ListDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files and directories
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), []byte("content3"), 0644)
	require.NoError(t, err)

	config := &domain.Config{
		ConfigPath: tmpDir,
	}
	service := NewService(config, zerolog.Nop())

	tests := []struct {
		name          string
		path          string
		expectedCount int
		expectedError bool
		validate      func(*testing.T, []domain.FileEntry)
	}{
		{
			name:          "list directory with explicit path",
			path:          tmpDir,
			expectedCount: 3, // file1.txt, file2.txt, subdir
			expectedError: false,
			validate: func(t *testing.T, entries []domain.FileEntry) {
				assert.Equal(t, 3, len(entries))
				// Directories should come first
				assert.True(t, entries[0].IsDir)
				assert.Equal(t, "subdir", entries[0].Name)
			},
		},
		{
			name:          "list directory with empty path (uses config path)",
			path:          "",
			expectedCount: 3,
			expectedError: false,
		},
		{
			name:          "list non-existent directory",
			path:          filepath.Join(tmpDir, "nonexistent"),
			expectedError: true,
		},
		{
			name:          "list file (not a directory)",
			path:          filepath.Join(tmpDir, "file1.txt"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ListDir(context.Background(), tt.path)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectedCount > 0 {
					assert.Equal(t, tt.expectedCount, len(result))
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestService_ListDir_Cancellation(t *testing.T) {
	tmpDir := t.TempDir()

	config := &domain.Config{
		ConfigPath: tmpDir,
	}
	service := NewService(config, zerolog.Nop())

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := service.ListDir(ctx, tmpDir)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
}

func TestService_ListDir_Sorting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files and directories
	err := os.WriteFile(filepath.Join(tmpDir, "z_file.txt"), []byte("content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "a_file.txt"), []byte("content"), 0644)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpDir, "b_dir"), 0755)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpDir, "m_dir"), 0755)
	require.NoError(t, err)

	config := &domain.Config{
		ConfigPath: tmpDir,
	}
	service := NewService(config, zerolog.Nop())

	result, err := service.ListDir(context.Background(), tmpDir)
	require.NoError(t, err)
	require.Equal(t, 4, len(result))

	// Directories should come first, sorted
	assert.True(t, result[0].IsDir)
	assert.Equal(t, "b_dir", result[0].Name)
	assert.True(t, result[1].IsDir)
	assert.Equal(t, "m_dir", result[1].Name)

	// Files should come after, sorted
	assert.False(t, result[2].IsDir)
	assert.Equal(t, "a_file.txt", result[2].Name)
	assert.False(t, result[3].IsDir)
	assert.Equal(t, "z_file.txt", result[3].Name)
}

func TestService_ListLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	// Create log files
	err = os.WriteFile(filepath.Join(logDir, "app.log"), []byte("log content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logDir, "other.log"), []byte("old log"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logDir, "not-a-log.txt"), []byte("not a log"), 0644)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(logDir, "subdir"), 0755)
	require.NoError(t, err)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	tests := []struct {
		name          string
		expectedCount int
		expectedError bool
		validate      func(*testing.T, []domain.LogFile)
	}{
		{
			name:          "list log files",
			expectedCount: 2, // app.log and app.log.1, but not not-a-log.txt or subdir
			expectedError: false,
			validate: func(t *testing.T, logs []domain.LogFile) {
				assert.Equal(t, 2, len(logs))
				// Verify all entries are .log files
				for _, log := range logs {
					assert.Contains(t, log.Name, ".log")
					assert.NotEmpty(t, log.Path)
					assert.NotEmpty(t, log.SizeHuman)
					assert.NotEmpty(t, log.ModifiedAt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ListLogs(context.Background())
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectedCount > 0 {
					assert.Equal(t, tt.expectedCount, len(result))
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestService_ListLogs_Cancellation(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	// Create multiple log files to ensure iteration happens
	err = os.WriteFile(filepath.Join(logDir, "app.log"), []byte("log content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(logDir, "other.log"), []byte("more logs"), 0644)
	require.NoError(t, err)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	// Test context cancellation - cancel before calling to test early cancellation
	// The cancellation check happens in the loop, so if we cancel before the function
	// is called, it will be caught on the first iteration of the loop (since we have files)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := service.ListLogs(ctx)
	// The cancellation is checked in the loop, so with files present, it should be caught
	// on the first iteration. However, if os.ReadDir completes before the loop starts,
	// the cancellation might not be detected until the loop runs.
	// Since we have files, the loop will run and should detect cancellation.
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
}

func TestService_ListLogs_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	result, err := service.ListLogs(context.Background())
	assert.NoError(t, err)
	// Result can be nil (nil slice) or empty slice - both are valid in Go
	// We just need to ensure it's safe to use (len works on both)
	if result == nil {
		assert.Equal(t, 0, len(result))
	} else {
		assert.Equal(t, 0, len(result))
		assert.Equal(t, []domain.LogFile{}, result)
	}
}

func TestService_DownloadLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	logContent := "This is test log content\nLine 2\nLine 3"
	logFile := "test.log"
	logPath := filepath.Join(logDir, logFile)
	err = os.WriteFile(logPath, []byte(logContent), 0644)
	require.NoError(t, err)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	tests := []struct {
		name          string
		filename      string
		expectedError bool
		errContains   string
		validate      func(*testing.T, []byte)
	}{
		{
			name:          "download existing log file",
			filename:      logFile,
			expectedError: false,
			validate: func(t *testing.T, content []byte) {
				assert.Equal(t, logContent, string(content))
			},
		},
		{
			name:          "download non-existent log file",
			filename:      "nonexistent.log",
			expectedError: true,
			errContains:   "log file not found",
		},
		{
			name:          "download file with wrong extension",
			filename:      "not-a-log.txt",
			expectedError: true,
			errContains:   "invalid file extension",
		},
		{
			name:          "download file with path traversal attempt",
			filename:      "../other.log",
			expectedError: true,
			errContains:   "log file not found", // Should be sanitized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.DownloadLogFile(context.Background(), tt.filename)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
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

func TestService_DownloadLogFile_Cancellation(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := service.DownloadLogFile(ctx, "test.log")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
}

func TestService_ListLogs_FileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")
	err := os.Mkdir(logDir, 0755)
	require.NoError(t, err)

	// Create a log file with known size
	logContent := "test log content"
	logFile := "test.log"
	err = os.WriteFile(filepath.Join(logDir, logFile), []byte(logContent), 0644)
	require.NoError(t, err)

	// Wait a bit to ensure different modification times
	time.Sleep(10 * time.Millisecond)

	config := &domain.Config{
		LogPath: filepath.Join(logDir, "app.log"),
	}
	service := NewService(config, zerolog.Nop())

	result, err := service.ListLogs(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	log := result[0]
	assert.Equal(t, logFile, log.Name)
	assert.Equal(t, int64(len(logContent)), log.Size)
	assert.NotEmpty(t, log.SizeHuman)
	assert.NotEmpty(t, log.ModifiedAt)
	// Verify modified time is in expected format
	_, err = time.Parse("2006-01-02 15:04:05", log.ModifiedAt)
	assert.NoError(t, err, "ModifiedAt should be in expected format")
}
