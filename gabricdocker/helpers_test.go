package gabricdocker

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/logger"
)

func testContext(t *testing.T) context.Context {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return logger.NewLoggerContext(context.Background(), log)
}

func recordingBackend(reqs *[]gabric.ExecRequest) gabric.LocalBackendFunc {
	return gabric.LocalBackendFunc{
		ExecFunc: func(_ context.Context, req gabric.ExecRequest) error {
			*reqs = append(*reqs, req)
			return nil
		},
	}
}
