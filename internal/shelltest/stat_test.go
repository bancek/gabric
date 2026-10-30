//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestStatMode(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "stat -c %a /srv/file", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "644\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("stat", "-c", "%a", "/srv/file")
}

func TestStatRawModeAndOwner(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().SetOwner("/srv/file", "testuser", "staff"); err != nil {
		t.Fatalf("SetOwner() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "stat -L -c '%f:%U:%G' /srv/file", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "81a0:testuser:staff\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("stat", "-L", "-c", "%f:%U:%G", "/srv/file")
}

func TestStatModeAfterChmod(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chmod 600 /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("chmod = (%d, %v), want (0, nil)", code, err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "stat -c %a /srv/file", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("stat = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), "600\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestStatDirectory(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "stat -c %a /srv/app", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), "755\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestStatMissing(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "stat -c %a /srv/missing", nil, nil, &stderr)
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

func TestStatRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "cd /srv && stat -c %a file", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), "640\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestStatSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sudo stat -c %a /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("stat", "-c", "%a", "/srv/file")
	if !ok {
		t.Fatal("stat command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestStatLongFormat(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	tests := []string{
		"stat --format=%a /srv/file",
		"stat --format %a /srv/file",
	}

	for _, script := range tests {
		var stdout bytes.Buffer

		if code, err := core.RunScript(context.Background(), script, nil, &stdout, nil); err != nil || code != 0 {
			t.Errorf("%s: RunScript() = (%d, %v), want (0, nil)", script, code, err)
			continue
		}
		if got, want := stdout.String(), "644\n"; got != want {
			t.Errorf("%s: stdout = %q, want %q", script, got, want)
		}
	}
}

func TestStatUnsupportedFormat(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "stat -c %n /srv/file", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "unsupported format") {
		t.Errorf("stderr = %q, want unsupported format", stderr.String())
	}
}

func TestStatMissingFormat(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "stat /srv/file", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestStatMissingOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "stat -c %a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestStatSymlink(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().SetOwner("/srv/file", "testuser", "staff"); err != nil {
		t.Fatalf("SetOwner() error: %v", err)
	}
	if err := core.FS().Symlink("/srv/file", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "stat -L -c '%f:%U:%G' /srv/link", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
	if got, want := stdout.String(), "81a0:testuser:staff\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	stdout.Reset()

	if code, err := core.RunScript(context.Background(), "stat -c %f /srv/link", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
	if got, want := stdout.String(), "a1ff\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestStatDanglingSymlink(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().Symlink("/srv/missing", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "stat -L -c %a /srv/link", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "/srv/link") {
		t.Errorf("stderr = %q, want path", stderr.String())
	}
}

func TestStatMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/b", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	if code, err := core.RunScript(context.Background(), "stat -c %a /srv/a /srv/b", nil, &stdout, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if got, want := stdout.String(), "600\n644\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
