package gabrictest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bancek/gabric/reconcile"
)

func TestRunNode(t *testing.T) {
	g := New(t)

	node := reconcile.Data("value", func(context.Context) (string, error) {
		return "ok", nil
	}).Node()

	if got := g.RunNode(g.Context(), node); got != "ok" {
		t.Errorf("RunNode() = %q, want %q", got, "ok")
	}
}

func TestRunNodeErr(t *testing.T) {
	g := New(t)

	sentinel := errors.New("boom")
	node := reconcile.Data("value", func(context.Context) (string, error) {
		return "", sentinel
	}).Node()

	value, err := g.RunNodeErr(g.Context(), node)
	if !errors.Is(err, sentinel) {
		t.Errorf("RunNodeErr() error = %v, want %v", err, sentinel)
	}
	if value != "" {
		t.Errorf("RunNodeErr() value = %q, want empty", value)
	}
}

func TestRunNodeFails(t *testing.T) {
	fake := &fakeTB{}
	g := New(fake)

	node := reconcile.Data("value", func(context.Context) (string, error) {
		return "", errors.New("boom")
	}).Node()

	g.RunNode(g.WithContext(context.Background()), node)

	if !strings.HasPrefix(fake.fatalf, "RunNode() error:") || !strings.Contains(fake.fatalf, "boom") {
		t.Errorf("RunNode() failure = %q, want a RunNode error containing %q", fake.fatalf, "boom")
	}
}

type fakeTB struct {
	testing.TB
	fatalf string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.fatalf = fmt.Sprintf(format, args...)
}
