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

func TestFSHandlersCdPwd(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "cd /srv/app && pwd", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "/srv/app\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("cd", "/srv/app")
	asserts.AssertExecuted("pwd")

	if cmd, ok := asserts.Find("pwd"); !ok || cmd.Dir != "/srv/app" {
		t.Errorf("pwd command = %+v, want dir /srv/app", cmd)
	}
}

func TestFSHandlersTestOps(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	tests := []struct {
		script string
		want   int
	}{
		{"test -e /srv/app/config", 0},
		{"test -f /srv/app/config", 0},
		{"test -d /srv/app", 0},
		{"test -e /srv/missing", 1},
		{"test -e /etc/passwd", 1},
		{"test -f /srv/app", 1},
	}

	for _, tt := range tests {
		code, err := core.RunScript(context.Background(), tt.script, nil, nil, nil)
		if err != nil {
			t.Errorf("%s: RunScript() error: %v", tt.script, err)
			continue
		}
		if code != tt.want {
			t.Errorf("%s: exit code = %d, want %d", tt.script, code, tt.want)
		}
	}
}

func TestFSHandlersTestOpsSymlink(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().Symlink("/srv/app/file", "/srv/app/file-link"); err != nil {
		t.Fatalf("Symlink(file) error: %v", err)
	}
	if err := core.FS().Symlink("/srv/app", "/srv/app/dir-link"); err != nil {
		t.Fatalf("Symlink(dir) error: %v", err)
	}
	if err := core.FS().Symlink("/srv/missing", "/srv/app/dangling"); err != nil {
		t.Fatalf("Symlink(dangling) error: %v", err)
	}

	tests := []struct {
		script string
		want   int
	}{
		{"test -e /srv/app/file-link", 0},
		{"test -f /srv/app/file-link", 0},
		{"test -d /srv/app/file-link", 1},
		{"test -L /srv/app/file-link", 0},
		{"test -e /srv/app/dir-link", 0},
		{"test -d /srv/app/dir-link", 0},
		{"test -f /srv/app/dir-link", 1},
		{"test -L /srv/app/dir-link", 0},
		{"test -e /srv/app/dangling", 1},
		{"test -f /srv/app/dangling", 1},
		{"test -d /srv/app/dangling", 1},
		{"test -L /srv/app/dangling", 0},
		{"test -e /srv/app/file", 0},
		{"test -L /srv/app/file", 1},
	}

	for _, tt := range tests {
		code, err := core.RunScript(context.Background(), tt.script, nil, nil, nil)
		if err != nil {
			t.Errorf("%s: RunScript() error: %v", tt.script, err)
			continue
		}
		if code != tt.want {
			t.Errorf("%s: exit code = %d, want %d", tt.script, code, tt.want)
		}
	}
}

func TestFSHandlersNestedSh(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sh -c 'cd /srv && pwd'", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "/srv\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestFSHandlersRelativePath(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "cd /srv/app && test -e ../app/config", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestFSHandlersCdMissing(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, _ := core.RunScript(context.Background(), "cd /missing", nil, nil, &stderr)
	if code == 0 {
		t.Error("exit code = 0, want non-zero")
	}

	if !strings.Contains(stderr.String(), "/missing") {
		t.Errorf("stderr = %q, want /missing", stderr.String())
	}
}

func TestFSHandlersGlob(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/b", []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "echo /srv/app/*", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "/srv/app/a /srv/app/b\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestFSHandlersDevNull(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "cd /srv/app >/dev/null && echo hi", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestFSHandlersWriteRedirect(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "echo hi > /srv/out", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	got, err := core.FS().ReadFile("/srv/out")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(got) != "hi\n" {
		t.Errorf("content = %q, want %q", got, "hi\n")
	}
}

func TestFSHandlersAppendRedirect(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	script := "echo one > /srv/out; echo two >> /srv/out"

	if code, err := core.RunScript(context.Background(), script, nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	got, err := core.FS().ReadFile("/srv/out")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(got) != "one\ntwo\n" {
		t.Errorf("content = %q, want %q", got, "one\ntwo\n")
	}
}

func TestFSHandlersReadRedirect(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout bytes.Buffer

	script := `echo hi > /srv/out; read x < /srv/out; echo "$x"`

	code, err := core.RunScript(context.Background(), script, nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestFSHandlersRelativeWrite(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "cd /srv/app && echo hi > out", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	got, err := core.FS().ReadFile("/srv/app/out")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(got) != "hi\n" {
		t.Errorf("content = %q, want %q", got, "hi\n")
	}
}

func TestFSHandlersWriteMissingParent(t *testing.T) {
	core := New()

	code, _ := core.RunScript(context.Background(), "echo hi > /missing/out", nil, nil, nil)
	if code == 0 {
		t.Error("exit code = 0, want non-zero")
	}

	if _, err := core.FS().Stat("/missing/out"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want ErrNotExist", err)
	}
}

func TestFSHandlersTruncateRedirect(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	script := "echo long > /srv/out; echo x > /srv/out"

	if code, err := core.RunScript(context.Background(), script, nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	got, err := core.FS().ReadFile("/srv/out")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(got) != "x\n" {
		t.Errorf("content = %q, want %q", got, "x\n")
	}
}
