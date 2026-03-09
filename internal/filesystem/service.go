package filesystem

import (
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/domain"
	"os"
	"path/filepath"
	"sort"
)

type Service interface {
	ListDir(ctx context.Context, path string) ([]domain.FileEntry, error)
	ListLogs(ctx context.Context) ([]domain.LogFile, error)
	DownloadLogFile(ctx context.Context, filename string) ([]byte, error)
}

type service struct {
	config *domain.Config
	log    zerolog.Logger
}

func NewService(config *domain.Config, log zerolog.Logger) Service {
	return &service{
		config: config,
		log:    log.With().Str("module", "filesystem").Logger(),
	}
}

func (s *service) ListDir(ctx context.Context, path string) ([]domain.FileEntry, error) {
	select {
	case <-ctx.Done():
		s.log.Warn().Msg("list dir cancelled before start")
		return nil, ctx.Err()
	default:
	}

	if path == "" {
		path = s.config.ConfigPath
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		s.log.Error().Err(err).Msg("invalid path")
		return nil, err
	}

	s.log.Debug().Str("path", path).Msg("listing directory")

	entries, err := os.ReadDir(absPath)
	if err != nil {
		s.log.Error().Err(err).Str("path", path).Msg("failed to read dir")
		return nil, err
	}

	var files []domain.FileEntry
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			s.log.Warn().Str("path", path).Msg("list dir cancelled during iteration")
			return nil, ctx.Err()
		default:
		}

		fullPath := filepath.Join(absPath, entry.Name())
		files = append(files, domain.FileEntry{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: entry.IsDir(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	s.log.Debug().Int("count", len(files)).Msg("directory listing complete")
	return files, nil
}

func (s *service) ListLogs(ctx context.Context) ([]domain.LogFile, error) {
	logPath := filepath.Dir(s.config.LogPath)
	entries, err := os.ReadDir(logPath)
	if err != nil {
		s.log.Error().Err(err).Str("path", s.config.LogPath).Msg("failed to read log directory")
		return nil, err
	}

	var logs []domain.LogFile

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			s.log.Warn().Msg("log listing cancelled during iteration")
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			s.log.Error().Err(err).Str("file", entry.Name()).Msg("failed to get file info")
			continue
		}

		fullPath, err := filepath.Abs(filepath.Join(logPath, entry.Name()))
		if err != nil {
			s.log.Error().Err(err).Str("file", entry.Name()).Msg("failed to get absolute path")
			continue
		}

		logs = append(logs, domain.LogFile{
			Name:       entry.Name(),
			Path:       fullPath,
			Size:       info.Size(),
			SizeHuman:  humanize.Bytes(uint64(info.Size())),
			ModifiedAt: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return logs, nil
}

func (s *service) DownloadLogFile(ctx context.Context, filePath string) ([]byte, error) {
	select {
	case <-ctx.Done():
		s.log.Warn().Msg("download log file cancelled before start")
		return nil, ctx.Err()
	default:
	}

	logPath := filepath.Dir(s.config.LogPath)
	filePath = filepath.Join(logPath, filePath)

	if filepath.Ext(filePath) != ".log" {
		s.log.Error().Str("filename", filePath).Msg("invalid file extension")
		return nil, fmt.Errorf("invalid file extension")
	}

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.log.Error().Str("path", filePath).Msg("log file not found")
			return nil, fmt.Errorf("log file not found")
		}
		s.log.Error().Err(err).Str("path", filePath).Msg("failed to check log file")
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		s.log.Error().Err(err).Str("path", filePath).Msg("failed to read log file")
		return nil, err
	}

	s.log.Trace().Str("path", filePath).Int("size", len(content)).Msg("log file downloaded")
	return content, nil
}
