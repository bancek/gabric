//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestChownFile(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "chown root:root /srv/file", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Stat("/srv/file"); err != nil {
		t.Errorf("Stat() error: %v", err)
	}

	user, group, err := core.FS().Owner("/srv/file")
	if err != nil {
		t.Fatalf("Owner() error: %v", err)
	}
	if user != "root" || group != "root" {
		t.Errorf("owner = %q:%q, want root:root", user, group)
	}

	NewAssertions(t, core.Commands).AssertExecuted("chown", "root:root", "/srv/file")
}

func TestChownUser(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chown root /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}

func TestChownGroup(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chown :root /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}

func TestChownMissing(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "chown root /srv/missing", nil, nil, &stderr)
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

func TestChownInvalidOwner(t *testing.T) {
	tests := []string{
		"chown : /srv/file",
		"chown root:root:extra /srv/file",
	}

	for _, script := range tests {
		core := New()

		if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		var stderr bytes.Buffer

		code, err := core.RunScript(context.Background(), script, nil, nil, &stderr)
		if err != nil {
			t.Errorf("%s: RunScript() error: %v", script, err)
			continue
		}
		if code != 1 {
			t.Errorf("%s: exit code = %d, want 1", script, code)
		}
		if !strings.Contains(stderr.String(), "invalid owner") {
			t.Errorf("%s: stderr = %q, want invalid owner", script, stderr.String())
		}
	}
}

func TestChownMissingOperand(t *testing.T) {
	tests := []string{"chown", "chown root"}

	for _, script := range tests {
		core := New()

		code, err := core.RunScript(context.Background(), script, nil, nil, nil)
		if err != nil {
			t.Errorf("%s: RunScript() error: %v", script, err)
			continue
		}
		if code != 1 {
			t.Errorf("%s: exit code = %d, want 1", script, code)
		}
	}
}

func TestChownInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "chown -R root /srv/file", nil, nil, &stderr)
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

func TestChownRelative(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "cd /srv && chown root file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}

func TestChownSudo(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "sudo chown root:root /srv/file", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("chown", "root:root", "/srv/file")
	if !ok {
		t.Fatal("chown command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestChownMultiple(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	for _, p := range []string{"/srv/a", "/srv/b"} {
		if err := core.FS().WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
	}

	if code, err := core.RunScript(context.Background(), "chown root /srv/a /srv/b", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}

func TestChownDirectory(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if code, err := core.RunScript(context.Background(), "chown root /srv/app", nil, nil, nil); err != nil || code != 0 {
		t.Fatalf("RunScript() = (%d, %v), want (0, nil)", code, err)
	}
}
