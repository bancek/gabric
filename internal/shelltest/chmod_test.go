//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"io/fs"
	"strings"
	"testing"
)

func TestChmodFile(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "chmod 600 /srv/file", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	info, err := core.FS().Stat("/srv/file")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("chmod", "600", "/srv/file")
}

func TestChmodDirectory(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chmod 700 /srv/app", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	info, err := core.FS().Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if !info.IsDir() {
		t.Error("IsDir() = false, want true")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestChmodMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	for _, p := range []string{"/srv/a", "/srv/b"} {
		if err := core.FS().WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
	}

	if code, err := core.RunScript(context.Background(), "chmod 640 /srv/a /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	for _, p := range []string{"/srv/a", "/srv/b"} {
		info, err := core.FS().Stat(p)
		if err != nil {
			t.Fatalf("Stat(%s) error: %v", p, err)
		}
		if got, want := info.Mode().Perm(), fs.FileMode(0o640); got != want {
			t.Errorf("%s mode = %v, want %v", p, got, want)
		}
	}
}

func TestChmodMissing(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "chmod 600 /srv/missing", nil, nil, &stderr)
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

func TestChmodInvalidMode(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "chmod xyz /srv/file", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "invalid mode") {
		t.Errorf("stderr = %q, want invalid mode", stderr.String())
	}
}

func TestChmodRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "cd /srv && chmod 600 file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	info, err := core.FS().Stat("/srv/file")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestChmodSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sudo chmod 600 /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("chmod", "600", "/srv/file")
	if !ok {
		t.Fatal("chmod command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestChmodOptionTerminator(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chmod -- 600 /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}

func TestChmodSpecialBits(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chmod 4755 /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	info, err := core.FS().Stat("/srv/file")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode()&fs.ModeSetuid == 0 {
		t.Error("setuid bit not set")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o755); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestChmodMissingOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "chmod 600", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestChmodInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "chmod -R 600 /srv/file", nil, nil, &stderr)
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
