package gabric

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bancek/gabric/logger"
)

func testContext(t *testing.T) context.Context {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return logger.NewLoggerContext(context.Background(), log)
}

func testContextBuffer(t *testing.T) (context.Context, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}

	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return logger.NewLoggerContext(context.Background(), log), buf
}
