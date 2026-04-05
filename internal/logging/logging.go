package logging

import (
	"io"
	"log/slog"
	"strings"

	"github.com/Matthew11K/Comments-Service/internal/config"
)

type Error struct {
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return e.Field + ": " + e.Message
}

func New(cfg config.LoggingConfig, output io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(cfg.Format) {
	case "json":
		return slog.New(slog.NewJSONHandler(output, options)), nil
	case "text":
		return slog.New(slog.NewTextHandler(output, options)), nil
	default:
		return nil, &Error{
			Field:   "logging.format",
			Message: "must be text or json",
		}
	}
}

func parseLevel(raw string) (slog.Leveler, error) {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, &Error{
			Field:   "logging.level",
			Message: "must be debug, info, warn, or error",
		}
	}
}
