//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

const (
	sha256Hello = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	sha256Empty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func TestSha256Sum(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum /srv/file", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), sha256Hello+"  /srv/file\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("sha256sum", "/srv/file")
}

func TestSha256SumMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/a", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/b", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum /srv/a /srv/b", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	want := sha256Hello + "  /srv/a\n" + sha256Hello + "  /srv/b\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumMissing(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum /srv/missing", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "/srv/missing") {
		t.Errorf("stderr = %q, want path", stderr.String())
	}
}

func TestSha256SumStdin(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum", strings.NewReader("hello"), &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), sha256Hello+"  -\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumDash(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum -", strings.NewReader("hello"), &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), sha256Hello+"  -\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumDevNull(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum /dev/null", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), sha256Empty+"  /dev/null\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "cd /srv && sha256sum file", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), sha256Hello+"  file\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sudo sha256sum /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("sha256sum", "/srv/file")
	if !ok {
		t.Fatal("sha256sum command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestSha256SumOptionTerminator(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/-n", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "sha256sum -- /srv/-n", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), sha256Hello+"  /srv/-n\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestSha256SumInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sha256sum -n /srv/file", nil, nil, &stderr)
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
