package logging

import (
	"io"
	"log/slog"
	"os"
)

// Setup configures the global slog logger.
// JSON handler in production (RAILWAY_ENVIRONMENT or FLY_APP_NAME set).
// Text handler in local development.
// If w is nil, defaults to os.Stderr.
func Setup(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	var handler slog.Handler
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("FLY_APP_NAME") != "" {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(handler))
}
