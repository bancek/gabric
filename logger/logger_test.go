package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/bancek/gabric/logger"
)

func testLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}

	log := slog.New(logger.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return log, buf
}

func TestTextHandlerDedup(t *testing.T) {
	log, buf := testLogger()

	log.With("task", "ci").With("task", "fmt").Info("message")

	out := buf.String()

	if !strings.Contains(out, "task=fmt") {
		t.Errorf("output = %q, want it to contain %q", out, "task=fmt")
	}

	if strings.Contains(out, "task=ci") {
		t.Errorf("output = %q, want it to not contain %q", out, "task=ci")
	}
}

func TestTextHandlerGroup(t *testing.T) {
	log, buf := testLogger()

	log.WithGroup("group").With("key", "value").Info("message")

	out := buf.String()

	if !strings.Contains(out, "group.key=value") {
		t.Errorf("output = %q, want it to contain %q", out, "group.key=value")
	}
}

// mutableValue is a slog.LogValuer whose value can change between With and the
// log call; it verifies resolution is deferred to Handle.
type mutableValue struct {
	resolved bool
	value    string
}

func (m *mutableValue) LogValue() slog.Value {
	if !m.resolved {
		return slog.StringValue("pending")
	}
	return slog.StringValue(m.value)
}

func TestTextHandlerLogValuerDeferred(t *testing.T) {
	log, buf := testLogger()

	v := &mutableValue{value: "ready"}
	l := log.With("v", v)

	l.Info("first")
	v.resolved = true
	l.Info("second")

	out := buf.String()

	if !strings.Contains(out, "v=pending") {
		t.Errorf("output = %q, want it to contain %q", out, "v=pending")
	}

	if !strings.Contains(out, "v=ready") {
		t.Errorf("output = %q, want it to contain %q", out, "v=ready")
	}
}

func TestLoggerContext(t *testing.T) {
	log, _ := testLogger()

	ctx := logger.NewLoggerContext(context.Background(), log)

	if logger.LoggerFromContext(ctx) != log {
		t.Error("LoggerFromContext() did not return the logger from context")
	}
}

func TestLoggerContextFallback(t *testing.T) {
	if got := logger.LoggerFromContext(context.Background()); got != slog.Default() {
		t.Error("LoggerFromContext() without a logger should return slog.Default()")
	}
}

func TestHasLogger(t *testing.T) {
	if logger.HasLogger(context.Background()) {
		t.Error("HasLogger() = true without a logger, want false")
	}

	log, _ := testLogger()

	if !logger.HasLogger(logger.NewLoggerContext(context.Background(), log)) {
		t.Error("HasLogger() = false with a logger, want true")
	}
}
