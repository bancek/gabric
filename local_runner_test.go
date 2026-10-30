package gabric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestNewLocalRunner(t *testing.T) {
	var got ExecRequest

	backend := LocalBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req

			if _, err := io.WriteString(req.Stdout, "out"); err != nil {
				return err
			}

			_, err := io.WriteString(req.Stderr, "err")

			return err
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)

	var stdout, stderr bytes.Buffer

	stdin := strings.NewReader("in")

	runner := NewLocalRunner(func(cmd string) []string {
		return []string{"/bin/sh", "-c", cmd}
	})

	if err := runner(ctx, "echo hi", stdin, &stdout, &stderr); err != nil {
		t.Fatalf("runner() error: %v", err)
	}

	if want := []string{"/bin/sh", "-c", "echo hi"}; !slices.Equal(got.Cmd, want) {
		t.Errorf("Cmd = %v, want %v", got.Cmd, want)
	}
	if got.Stdin != stdin {
		t.Error("Stdin was not passed through")
	}
	if got.Stdout != &stdout {
		t.Error("Stdout was not passed through")
	}
	if got.Stderr != &stderr {
		t.Error("Stderr was not passed through")
	}
	if got := stdout.String(); got != "out" {
		t.Errorf("stdout = %q, want %q", got, "out")
	}
	if got := stderr.String(); got != "err" {
		t.Errorf("stderr = %q, want %q", got, "err")
	}
}

func TestNewLocalRunnerPassesThroughError(t *testing.T) {
	sentinel := errors.New("boom")

	ctx := NewLocalBackendContext(testContext(t), LocalBackendFunc{
		ExecFunc: func(context.Context, ExecRequest) error {
			return sentinel
		},
	})

	runner := NewLocalRunner(func(string) []string { return nil })

	err := runner(ctx, "x", nil, io.Discard, io.Discard)
	if !errors.Is(err, sentinel) {
		t.Errorf("runner() error = %v, want %v", err, sentinel)
	}
}
