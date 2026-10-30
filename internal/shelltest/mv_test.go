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

func TestMVFile(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/a", []byte("hello"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "mv /srv/a /srv/b", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	b, err := core.FS().ReadFile("/srv/b")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", b, "hello")
	}

	info, err := core.FS().Stat("/srv/b")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o640); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	if _, err := core.FS().Stat("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(old) error = %v, want ErrNotExist", err)
	}

	NewAssertions(t, core.Commands).AssertExecuted("mv", "/srv/a", "/srv/b")
}

func TestMVOverwrite(t *testing.T) {
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

	if code, err := core.RunScript(context.Background(), "mv /srv/a /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	b, err := core.FS().ReadFile("/srv/b")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(b) != "a" {
		t.Errorf("content = %q, want %q", b, "a")
	}
}

func TestMVDirectory(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/dir/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mv /srv/dir /srv/dir2", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	b, err := core.FS().ReadFile("/srv/dir2/a")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(b) != "a" {
		t.Errorf("content = %q, want %q", b, "a")
	}

	if _, err := core.FS().Stat("/srv/dir"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(old) error = %v, want ErrNotExist", err)
	}
}

func TestMVIntoDirectory(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/src", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mv /srv/src /srv/dir", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/dir/src"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMVMultiple(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/b", []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mv /srv/a /srv/b /srv/dir", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	for _, p := range []string{"/srv/dir/a", "/srv/dir/b"} {
		if _, err := core.FS().Stat(p); err != nil {
			t.Errorf("Stat(%s) error: %v", p, err)
		}
	}
}

func TestMVMultipleNotDirectory(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/b", []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/dest", []byte("d"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "mv /srv/a /srv/b /srv/dest", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestMVMissingSource(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "mv /srv/missing /srv/b", nil, nil, &stderr)
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

func TestMVMissingDestParent(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mv /srv/a /missing/b", nil, nil, nil); err != nil || code != 1 {
		t.Errorf("mv to missing parent = (%d, %v), want (1, nil)", code, err)
	}
}

func TestMVDirectoryIntoItself(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "mv /srv/dir /srv/dir/sub", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "cannot move") {
		t.Errorf("stderr = %q, want cannot move", stderr.String())
	}

	if _, err := core.FS().Stat("/srv/dir"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMVRelative(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "cd /srv && mv a b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/b"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMVNestedSh(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sh -c 'mv /srv/a /srv/b'", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/b"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMVSudo(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sudo mv /srv/a /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("mv", "/srv/a", "/srv/b")
	if !ok {
		t.Fatal("mv command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestMVOptionTerminator(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/-n", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "mv -- /srv/-n /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	if _, err := core.FS().Stat("/srv/b"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}
}

func TestMVInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "mv -f /srv/a /srv/b", nil, nil, &stderr)
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

func TestMVMissingOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "mv /srv/a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}
