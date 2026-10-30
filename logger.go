package gabric

import (
	"context"
	"log/slog"

	"github.com/bancek/gabric/logger"
)

// Logger returns the logger from ctx. If no logger is in context it falls back
// to the default logger (slog.Default).
func Logger(ctx context.Context) *slog.Logger {
	return logger.LoggerFromContext(ctx)
}
