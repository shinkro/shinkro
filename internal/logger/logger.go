package logger

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/pkg/sse"
	"gopkg.in/natefinch/lumberjack.v2"
)

type dynamicLevelWriter struct {
	writer io.Writer
	mu     sync.RWMutex
	level  zerolog.Level
}

func (d *dynamicLevelWriter) Write(p []byte) (n int, err error) {
	return d.writer.Write(p)
}

func (d *dynamicLevelWriter) WriteLevel(l zerolog.Level, p []byte) (n int, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if l >= d.level {
		return d.writer.Write(p)
	}
	return len(p), nil
}

func (d *dynamicLevelWriter) SetLevel(level zerolog.Level) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.level = level
}

type Logger struct {
	zerolog.Logger
	levelWriter *dynamicLevelWriter
	writer      io.Writer
	mu          sync.Mutex
}

var instance *Logger

func NewLogger(c *domain.Config) *Logger {
	var defaultWriter io.Writer = os.Stderr

	if c.Version == "dev" {
		defaultWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	}

	lumberlog := &lumberjack.Logger{
		Filename:   c.LogPath,
		MaxSize:    c.LogMaxSize,
		MaxBackups: c.LogMaxBackups,
	}

	mw := io.MultiWriter(
		defaultWriter,
		lumberlog,
	)

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	level := parseLogLevel(c.LogLevel)
	dynWriter := &dynamicLevelWriter{
		writer: mw,
		level:  level,
	}

	log := zerolog.New(dynWriter).With().Timestamp().Logger()

	instance = &Logger{
		Logger:      log,
		writer:      mw,
		levelWriter: dynWriter,
	}

	return instance
}

func parseLogLevel(level string) zerolog.Level {
	switch level {
	case "TRACE":
		return zerolog.TraceLevel
	case "DEBUG":
		return zerolog.DebugLevel
	case "ERROR":
		return zerolog.ErrorLevel
	case "INFO":
		return zerolog.InfoLevel
	default:
		return zerolog.InfoLevel
	}
}

func (l *Logger) SetLogLevel(level string) error {
	parsed := parseLogLevel(level)
	l.levelWriter.SetLevel(parsed)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.Logger = zerolog.New(l.levelWriter).With().Timestamp().Logger()

	return nil
}

func (l *Logger) RegisterSSEWriter(sseServer *sse.Server) {
	// Use the embedded *r3labs/sse/v2.Server for compatibility with SSEWriter
	// Access the embedded field directly
	w := NewSSEWriter(sseServer.Server)
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Add SSE writer to the multiwriter
	l.writer = io.MultiWriter(l.writer, w)
	l.levelWriter.writer = l.writer
	l.Logger = zerolog.New(l.levelWriter).With().Timestamp().Logger()
}

func GetInstance() *Logger {
	return instance
}
