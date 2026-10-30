//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCat(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat /srv/app/config", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hello"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}

	NewAssertions(t, core.Commands).AssertExecuted("cat", "/srv/app/config")
}

func TestCatMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/b", []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat /srv/a /srv/b", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "ab"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatMissingAmongMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat /srv/a /srv/missing", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if got, want := stdout.String(), "a"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if !strings.Contains(stderr.String(), "/srv/missing") {
		t.Errorf("stderr = %q, want path", stderr.String())
	}
}

func TestCatDirectory(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat /srv", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "is a directory") {
		t.Errorf("stderr = %q, want is a directory", stderr.String())
	}
}

func TestCatDevNull(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat /dev/null", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got := stdout.String(); got != "" {
		t.Errorf("stdout = %q, want empty", got)
	}
}

func TestCatStdin(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat", strings.NewReader("hi"), &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatDash(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat -", strings.NewReader("hi"), &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatNoArgsEmptyStdin(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "cat", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestCatRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cd /srv/app && cat config", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hello"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatNestedSh(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sh -c 'cat /srv/app/config'", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hello"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo cat /srv/app/config", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hello"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("cat", "/srv/app/config")
	if !ok {
		t.Fatal("cat command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestCatOptionTerminator(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/-n", []byte("dash"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat -- /srv/-n", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "dash"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestCatInvalidOption(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "cat -n /srv", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "invalid option") {
		t.Errorf("stderr = %q, want invalid option", stderr.String())
	}
}
