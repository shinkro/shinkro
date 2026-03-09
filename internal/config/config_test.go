package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinkro/shinkro/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppConfig_ProcessLines(t *testing.T) {
	tests := []struct {
		name     string
		config   *AppConfig
		lines    []string
		validate func(*testing.T, []string)
	}{
		{
			name: "update existing CheckForUpdates",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: true,
					LogLevel:        "DEBUG",
					LogPath:         "/var/log/shinkro.log",
				},
			},
			lines: []string{
				"Host = \"127.0.0.1\"",
				"CheckForUpdates = false",
				"LogLevel = \"INFO\"",
			},
			validate: func(t *testing.T, result []string) {
				assert.Contains(t, strings.Join(result, "\n"), "CheckForUpdates = true")
				assert.Contains(t, strings.Join(result, "\n"), `LogLevel = "DEBUG"`)
			},
		},
		{
			name: "append missing CheckForUpdates",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: true,
					LogLevel:        "DEBUG",
					LogPath:         "",
				},
			},
			lines: []string{
				"Host = \"127.0.0.1\"",
			},
			validate: func(t *testing.T, result []string) {
				content := strings.Join(result, "\n")
				assert.Contains(t, content, "CheckForUpdates = true")
				assert.Contains(t, content, `LogLevel = "DEBUG"`)
			},
		},
		{
			name: "handle empty LogPath",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: true,
					LogLevel:        "INFO",
					LogPath:         "",
				},
			},
			lines: []string{
				"LogPath = \"\"",
			},
			validate: func(t *testing.T, result []string) {
				content := strings.Join(result, "\n")
				assert.Contains(t, content, "#LogPath = \"\"")
			},
		},
		{
			name: "preserve existing LogPath value when empty",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: true,
					LogLevel:        "INFO",
					LogPath:         "",
				},
			},
			lines: []string{
				"LogPath = \"/existing/path.log\"",
			},
			validate: func(t *testing.T, result []string) {
				content := strings.Join(result, "\n")
				// Should preserve existing value
				assert.Contains(t, content, "LogPath = \"/existing/path.log\"")
			},
		},
		{
			name: "update LogPath with value",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: true,
					LogLevel:        "INFO",
					LogPath:         "/new/path.log",
				},
			},
			lines: []string{
				"LogPath = \"/old/path.log\"",
			},
			validate: func(t *testing.T, result []string) {
				content := strings.Join(result, "\n")
				assert.Contains(t, content, "LogPath = \"/new/path.log\"")
			},
		},
		{
			name: "append all missing values",
			config: &AppConfig{
				Config: &domain.Config{
					CheckForUpdates: false,
					LogLevel:        "ERROR",
					LogPath:         "/log/path.log",
				},
			},
			lines: []string{
				"Host = \"127.0.0.1\"",
			},
			validate: func(t *testing.T, result []string) {
				content := strings.Join(result, "\n")
				assert.Contains(t, content, "CheckForUpdates = false")
				assert.Contains(t, content, `LogLevel = "ERROR"`)
				assert.Contains(t, content, `LogPath = "/log/path.log"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use reflection or type assertion to access processLines
			// Since processLines is not exported, we need to test it through UpdateConfig
			// or make it testable. For now, let's test the public interface.
			result := tt.config.processLines(tt.lines)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestAppConfig_ParseEnv(t *testing.T) {
	// Save original env
	originalEnv := make(map[string]string)
	envVars := []string{
		"SHINKRO_HOST",
		"SHINKRO_PORT",
		"SHINKRO_BASE_URL",
		"SHINKRO_LOG_LEVEL",
		"SHINKRO_LOG_PATH",
		"SHINKRO_LOG_MAX_SIZE",
		"SHINKRO_LOG_MAX_BACKUPS",
		"SHINKRO_SESSION_SECRET",
		"SHINKRO_ENCRYPTION_KEY",
		"SHINKRO_CHECK_FOR_UPDATES",
	}

	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *AppConfig)
	}{
		{
			name: "set all env vars",
			envVars: map[string]string{
				"SHINKRO_HOST":              "0.0.0.0",
				"SHINKRO_PORT":              "8080",
				"SHINKRO_BASE_URL":          "/app",
				"SHINKRO_LOG_LEVEL":         "DEBUG",
				"SHINKRO_LOG_PATH":          "/var/log/shinkro.log",
				"SHINKRO_LOG_MAX_SIZE":      "100",
				"SHINKRO_LOG_MAX_BACKUPS":   "10",
				"SHINKRO_SESSION_SECRET":    "secret123",
				"SHINKRO_ENCRYPTION_KEY":    "key123",
				"SHINKRO_CHECK_FOR_UPDATES": "false",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.Equal(t, "0.0.0.0", cfg.Config.Host)
				assert.Equal(t, 8080, cfg.Config.Port)
				assert.Equal(t, "/app", cfg.Config.BaseUrl)
				assert.Equal(t, "DEBUG", cfg.Config.LogLevel)
				assert.Equal(t, "/var/log/shinkro.log", cfg.Config.LogPath)
				assert.Equal(t, 100, cfg.Config.LogMaxSize)
				assert.Equal(t, 10, cfg.Config.LogMaxBackups)
				assert.Equal(t, "secret123", cfg.Config.SessionSecret)
				assert.Equal(t, "key123", cfg.Config.EncryptionKey)
				assert.False(t, cfg.Config.CheckForUpdates)
			},
		},
		{
			name: "invalid port",
			envVars: map[string]string{
				"SHINKRO_PORT": "invalid",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				// Should keep default port
				assert.NotEqual(t, 0, cfg.Config.Port)
			},
		},
		{
			name: "zero port",
			envVars: map[string]string{
				"SHINKRO_PORT": "0",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				// Should keep default port (0 is not > 0)
				assert.NotEqual(t, 0, cfg.Config.Port)
			},
		},
		{
			name: "negative port",
			envVars: map[string]string{
				"SHINKRO_PORT": "-1",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				// Should keep default port
				assert.NotEqual(t, -1, cfg.Config.Port)
			},
		},
		{
			name: "check for updates true",
			envVars: map[string]string{
				"SHINKRO_CHECK_FOR_UPDATES": "true",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.Config.CheckForUpdates)
			},
		},
		{
			name: "check for updates false",
			envVars: map[string]string{
				"SHINKRO_CHECK_FOR_UPDATES": "false",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.False(t, cfg.Config.CheckForUpdates)
			},
		},
		{
			name: "check for updates case insensitive",
			envVars: map[string]string{
				"SHINKRO_CHECK_FOR_UPDATES": "TRUE",
			},
			validate: func(t *testing.T, cfg *AppConfig) {
				assert.True(t, cfg.Config.CheckForUpdates)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			cfg := &AppConfig{
				Config: &domain.Config{},
			}
			cfg.defaultConfig()
			cfg.parseEnv()

			if tt.validate != nil {
				tt.validate(t, cfg)
			}

			// Cleanup
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestAppConfig_DefaultConfig(t *testing.T) {
	cfg := &AppConfig{}
	cfg.defaultConfig()

	assert.NotNil(t, cfg.Config)
	assert.Equal(t, "dev", cfg.Config.Version)
	assert.Equal(t, "localhost", cfg.Config.Host)
	assert.Equal(t, 7011, cfg.Config.Port)
	assert.Equal(t, "/", cfg.Config.BaseUrl)
	assert.Equal(t, "INFO", cfg.Config.LogLevel)
	assert.Equal(t, 50, cfg.Config.LogMaxSize)
	assert.Equal(t, 3, cfg.Config.LogMaxBackups)
	assert.NotEmpty(t, cfg.Config.SessionSecret)
	assert.NotEmpty(t, cfg.Config.EncryptionKey)
	assert.True(t, cfg.Config.CheckForUpdates)
}

func TestAppConfig_WriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &AppConfig{}
	cfg.defaultConfig()

	tests := []struct {
		name          string
		configPath    string
		configFile    string
		expectedError bool
		validate      func(*testing.T, string)
	}{
		{
			name:          "create config file",
			configPath:    tmpDir,
			configFile:    "config.toml",
			expectedError: false,
			validate: func(t *testing.T, filePath string) {
				_, err := os.Stat(filePath)
				assert.NoError(t, err)
				content, err := os.ReadFile(filePath)
				assert.NoError(t, err)
				assert.Contains(t, string(content), "Host")
				assert.Contains(t, string(content), "SessionSecret")
			},
		},
		{
			name:          "create nested directory",
			configPath:    filepath.Join(tmpDir, "nested", "config"),
			configFile:    "config.toml",
			expectedError: false,
			validate: func(t *testing.T, filePath string) {
				_, err := os.Stat(filePath)
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.WriteConfig(tt.configPath, tt.configFile)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					filePath := filepath.Join(tt.configPath, tt.configFile)
					tt.validate(t, filePath)
				}
			}
		})
	}

	// Test that existing file is not overwritten
	t.Run("existing file not overwritten", func(t *testing.T) {
		existingPath := filepath.Join(tmpDir, "existing.toml")
		existingContent := "existing content"
		err := os.WriteFile(existingPath, []byte(existingContent), 0644)
		require.NoError(t, err)

		err = cfg.WriteConfig(tmpDir, "existing.toml")
		assert.NoError(t, err)

		content, err := os.ReadFile(existingPath)
		assert.NoError(t, err)
		assert.Equal(t, existingContent, string(content))
	})
}

