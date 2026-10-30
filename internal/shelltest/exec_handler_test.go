//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"mvdan.cc/sh/v3/interp"
)

func TestExecHandler(t *testing.T) {
	core := New()
	handler := core.ExecHandler(context.Background())

	var stdout, stderr bytes.Buffer

	code := handler("/bin/sh -c 'echo hi'", nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}

	NewAssertions(t, core.Commands).AssertExecuted("echo", "hi")
}

func TestExecHandlerSudoNested(t *testing.T) {
	core := New()
	handler := core.ExecHandler(context.Background())

	var stdout bytes.Buffer

	code := handler(`sudo sh -c "sh -c 'echo nested'"`, nil, &stdout, nil)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "nested\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("echo", "nested")
	if !ok {
		t.Fatal("echo command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestExecHandlerExitStatus(t *testing.T) {
	core := New()
	handler := core.ExecHandler(context.Background())

	if code := handler("sh -c 'exit 3'", nil, nil, nil); code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestExecHandlerUnknownCommand(t *testing.T) {
	core := New()
	handler := core.ExecHandler(context.Background())

	var stderr bytes.Buffer

	if code := handler("sh -c 'not-a-real-command'", nil, nil, &stderr); code != 127 {
		t.Errorf("exit code = %d, want 127", code)
	}

	NewAssertions(t, core.Commands).AssertExecuted("not-a-real-command")
}

func TestExecHandlerParseError(t *testing.T) {
	core := New()
	handler := core.ExecHandler(context.Background())

	var stderr bytes.Buffer

	if code := handler("if true; then", nil, nil, &stderr); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if stderr.Len() == 0 {
		t.Error("stderr is empty, want parse error")
	}
}

type contextKey struct{}

func TestExecHandlerContext(t *testing.T) {
	core := New()

	core.Register("show-context", Handler{
		Run: func(ctx context.Context, _ Command) error {
			_, err := fmt.Fprint(interp.HandlerCtx(ctx).Stdout, ctx.Value(contextKey{}))
			return err
		},
	})

	ctx := context.WithValue(context.Background(), contextKey{}, "value")
	handler := core.ExecHandler(ctx)

	var stdout bytes.Buffer

	if code := handler("show-context", nil, &stdout, nil); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "value"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
