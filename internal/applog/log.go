// Package applog owns application diagnostics independently of the UI and domain.
package applog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// Config controls bounded application logs. Zero limits select the defaults.
// A nil Level selects DEBUG in development and INFO otherwise.
type Config struct {
	Dev            bool
	Level          slog.Leveler
	LogDir         string
	MaxFileSize    int64
	Generations    int
	BufferCapacity int
	Stderr         io.Writer
}

// FromEnv reads application variables without adding desktop CLI flags.
func FromEnv(dev bool) Config {
	config := Config{Dev: dev, LogDir: os.Getenv("FEBUILDER_LOG_DIR")}
	switch strings.ToLower(os.Getenv("FEBUILDER_LOG_LEVEL")) {
	case "debug":
		config.Level = slog.LevelDebug
	case "info":
		config.Level = slog.LevelInfo
	case "warn":
		config.Level = slog.LevelWarn
	case "error":
		config.Level = slog.LevelError
	}
	return config
}

// Sink owns a logger, its recent snapshots and the optional persistent file.
// Close is safe to repeat after application workers have finished.
type Sink struct {
	Logger *slog.Logger
	LogDir string
	Buffer *RingBuffer
	file   *rotatingFile
}

func New(config Config) *Sink {
	if config.Level == nil {
		config.Level = slog.LevelInfo
		if config.Dev {
			config.Level = slog.LevelDebug
		}
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 1 << 20
	}
	if config.Generations <= 0 {
		config.Generations = 3
	}
	if config.BufferCapacity <= 0 {
		config.BufferCapacity = 200
	}
	sink := &Sink{
		Buffer: &RingBuffer{entries: make([]Entry, config.BufferCapacity)},
	}
	options := &slog.HandlerOptions{Level: config.Level}
	fallbackHandlers := []slog.Handler{
		slog.NewTextHandler(config.Stderr, options),
		&bufferHandler{buffer: sink.Buffer, level: config.Level},
	}
	combined := teeHandler{handlers: fallbackHandlers}
	// Sink failures bypass the file and remain visible even at ERROR level.
	report := func(err error) {
		record := slog.NewRecord(
			time.Now(),
			slog.LevelWarn,
			"log file unavailable",
			0,
		)
		record.AddAttrs(slog.Any("error", fault.LogValue(err)))
		for _, handler := range fallbackHandlers {
			_ = handler.Handle(context.Background(), record.Clone())
		}
	}
	directory := config.LogDir
	var err error
	if directory == "" {
		directory, err = os.UserCacheDir()
		if err == nil {
			directory = filepath.Join(directory, "febuildergba-mygo", "logs")
		}
	}
	if err == nil {
		sink.file, err = openRotatingFile(
			directory,
			config.MaxFileSize,
			config.Generations,
			report,
		)
	}
	if err != nil {
		report(err)
	} else {
		sink.LogDir = directory
		combined.handlers = append(
			combined.handlers,
			slog.NewJSONHandler(sink.file, options),
		)
	}
	sink.Logger = slog.New(combined)
	return sink
}

func (s *Sink) Close() error {
	if s.file == nil {
		return nil
	}
	return s.file.Close()
}

// RecordError classifies failures at application and editor boundaries.
func RecordError(operation string, err error) {
	if err == nil {
		return
	}
	level := slog.LevelError
	switch fault.CodeOf(err).Kind {
	case fault.Canceled:
		return
	case fault.InvalidInput:
		level = slog.LevelDebug
	case fault.Conflict, fault.Unsupported:
		level = slog.LevelWarn
	}
	if slog.Default().Enabled(context.Background(), level) {
		slog.LogAttrs(
			context.Background(),
			level,
			"operation failed",
			slog.String(
				"operation",
				operation,
			),
			slog.Any("error", fault.LogValue(err)),
		)
	}
}

// teeHandler preserves each destination's own level and attribute scope.
type teeHandler struct {
	handlers []slog.Handler
}

func (h teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			err = errors.Join(err, handler.Handle(ctx, record.Clone()))
		}
	}
	return err
}

func (h teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := teeHandler{handlers: make([]slog.Handler, len(h.handlers))}
	for i, handler := range h.handlers {
		next.handlers[i] = handler.WithAttrs(attrs)
	}
	return next
}

func (h teeHandler) WithGroup(name string) slog.Handler {
	next := teeHandler{handlers: make([]slog.Handler, len(h.handlers))}
	for i, handler := range h.handlers {
		next.handlers[i] = handler.WithGroup(name)
	}
	return next
}

// Keep output serialization and disk rotation inside the writer's lock.
var (
	_ io.WriteCloser = (*rotatingFile)(nil)
	_ slog.Handler   = (*bufferHandler)(nil)
	_ slog.Handler   = teeHandler{}
)
