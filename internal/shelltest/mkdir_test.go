//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestMkdir(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "mkdir /srv/app", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	info, err := core.FS().Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if !info.IsDir() {
		t.Error("IsDir() = false, want true")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o755); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	NewAssertions(t, core.Commands).AssertExecuted("mkdir", "/srv/app")
}

func TestMkdirRecursive(t *testing.T) {
	core := New()

	if code, err := core.RunScript(context.Background(), "mkdir -p /srv/a/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	for _, p := range []string{"/srv", "/srv/a", "/srv/a/b"} {
		info, err := core.FS().Stat(p)
		if err != nil {
			t.Errorf("Stat(%s) error: %v", p, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s IsDir() = false, want true", p)
		}
	}
}

func TestMkdirMissingParent(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "mkdir /srv/a/b", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "/srv/a/b") {
		t.Errorf("stderr = %q, want path", stderr.String())
	}

	if _, err := core.FS().Stat("/srv/a/b"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want ErrNotExist", err)
	}
}

func TestMkdirExisting(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mkdir /srv/app", nil, nil, nil); err != nil || code != 1 {
		t.Errorf("mkdir existing = (%d, %v), want (1, nil)", code, err)
	}

	if code, err := core.RunScript(context.Background(), "mkdir -p /srv/app", nil, nil, nil); err != nil || code != 0 {
		t.Errorf("mkdir -p existing = (%d, %v), want (0, nil)", code, err)
	}
}

func TestMkdirExistingFile(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mkdir /srv/file", nil, nil, nil); err != nil || code != 1 {
		t.Errorf("mkdir file = (%d, %v), want (1, nil)", code, err)
	}
}

func TestMkdirRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "cd /srv && mkdir app", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/app"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMkdirNestedSh(t *testing.T) {
	core := New()

	if code, err := core.RunScript(context.Background(), "sh -c 'mkdir -p /srv/a'", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/a"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMkdirSudo(t *testing.T) {
	core := New()

	if code, err := core.RunScript(context.Background(), "sudo mkdir -p /srv/a", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("mkdir", "-p", "/srv/a")
	if !ok {
		t.Fatal("mkdir command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestMkdirMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mkdir /srv/a /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	for _, p := range []string{"/srv/a", "/srv/b"} {
		if _, err := core.FS().Stat(p); err != nil {
			t.Errorf("Stat(%s) error: %v", p, err)
		}
	}
}

func TestMkdirMissingOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "mkdir", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestMkdirInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "mkdir -q", nil, nil, &stderr)
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
