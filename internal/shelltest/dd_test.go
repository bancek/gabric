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

func TestDDWrite(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "dd of=/srv/out", strings.NewReader("hello"), nil, nil)
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

	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}

	NewAssertions(t, core.Commands).AssertExecuted("dd", "of=/srv/out")
}

func TestDDRead(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "dd if=/srv/in", nil, &stdout, nil)
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

func TestDDCopy(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "dd if=/srv/in of=/srv/out", nil, nil, nil)
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

	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestDDMissingInput(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "dd if=/srv/missing of=/srv/out", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "/srv/missing") {
		t.Errorf("stderr = %q, want input path", stderr.String())
	}

	if _, err := core.FS().Stat("/srv/out"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(out) error = %v, want ErrNotExist", err)
	}
}

func TestDDMissingOutputParent(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "dd of=/missing/out", strings.NewReader("x"), nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if _, err := core.FS().Stat("/missing/out"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(out) error = %v, want ErrNotExist", err)
	}
}

func TestDDCount(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	script := "dd bs=3 count=1 if=/srv/in of=/srv/out"

	code, err := core.RunScript(context.Background(), script, nil, nil, nil)
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

	if string(got) != "abc" {
		t.Errorf("content = %q, want %q", got, "abc")
	}
}

func TestDDInvalidBS(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "dd bs=x of=/srv/out", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "invalid bs") {
		t.Errorf("stderr = %q, want invalid bs", stderr.String())
	}
}

func TestDDUnsupportedOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "dd conv=notrunc", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestDDDevNull(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "dd if=/srv/in of=/dev/null", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("dd to /dev/null = (%d, %v), want (0, nil)", code, err)
	}

	if code, err := core.RunScript(context.Background(), "dd if=/dev/null of=/srv/out", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("dd from /dev/null = (%d, %v), want (0, nil)", code, err)
	}

	got, err := core.FS().ReadFile("/srv/out")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("content = %q, want empty", got)
	}
}

func TestDDRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "cd /srv && dd if=in of=out", nil, nil, nil)
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

	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestDDNestedSh(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	script := "sh -c 'dd if=/srv/in of=/srv/out'"

	code, err := core.RunScript(context.Background(), script, nil, nil, nil)
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

	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestDDSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/in", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "sudo dd if=/srv/in of=/srv/out", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("dd", "if=/srv/in", "of=/srv/out")
	if !ok {
		t.Fatal("dd command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}
