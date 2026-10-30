package shelltest

import (
	"bytes"
	"context"
	"testing"
)

func TestUname(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "uname -s", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "Linux\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("uname", "-s")
}

func TestUnameUnsupported(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "uname -a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}
