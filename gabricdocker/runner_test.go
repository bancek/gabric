package gabricdocker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/bancek/gabric"
)

func TestRunner(t *testing.T) {
	var got gabric.ExecRequest

	backend := gabric.LocalBackendFunc{
		ExecFunc: func(_ context.Context, req gabric.ExecRequest) error {
			got = req

			content, err := io.ReadAll(req.Stdin)
			if err != nil {
				return err
			}
			if _, err := req.Stdout.Write(content); err != nil {
				return err
			}

			_, err = io.WriteString(req.Stderr, "err")

			return err
		},
	}

	ctx := gabric.NewLocalBackendContext(testContext(t), backend)
	ctx = NewContainerContext(ctx, "app")

	var stdout, stderr bytes.Buffer

	err := Runner()(ctx, "echo hi", strings.NewReader("in"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Runner() error: %v", err)
	}

	if want := []string{"docker", "exec", "-i", "app", "/bin/sh", "-c", "echo hi"}; !slices.Equal(got.Cmd, want) {
		t.Errorf("Cmd = %v, want %v", got.Cmd, want)
	}
	if got.Dir != "" {
		t.Errorf("Dir = %q, want empty", got.Dir)
	}
	if stdout.String() != "in" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "in")
	}
	if stderr.String() != "err" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "err")
	}
}

func TestRunnerEmptyContainer(t *testing.T) {
	err := Runner()(testContext(t), "true", nil, io.Discard, io.Discard)
	if !errors.Is(err, ErrNoContainer) {
		t.Fatalf("Runner() error = %v, want ErrNoContainer", err)
	}
}

func TestRunnerReadsCurrentContainer(t *testing.T) {
	var reqs []gabric.ExecRequest

	backend := recordingBackend(&reqs)
	base := gabric.NewLocalBackendContext(testContext(t), backend)

	runner := Runner()

	if err := runner(NewContainerContext(base, "one"), "true", nil, io.Discard, io.Discard); err != nil {
		t.Fatalf("Runner(one) error: %v", err)
	}
	if err := runner(NewContainerContext(base, "two"), "true", nil, io.Discard, io.Discard); err != nil {
		t.Fatalf("Runner(two) error: %v", err)
	}

	if len(reqs) != 2 {
		t.Fatalf("runner calls = %d, want 2", len(reqs))
	}
	if reqs[0].Cmd[3] != "one" {
		t.Errorf("first container = %q, want one", reqs[0].Cmd[3])
	}
	if reqs[1].Cmd[3] != "two" {
		t.Errorf("second container = %q, want two", reqs[1].Cmd[3])
	}
}
