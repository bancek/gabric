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

func TestRMFile(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "rm /srv/a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Lstat("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}

	NewAssertions(t, core.Commands).AssertExecuted("rm", "/srv/a")
}

func TestRMDirectoryRequiresRecursive(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/dir/a", []byte("a"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "rm /srv/dir", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if _, err := core.FS().Stat("/srv/dir"); err != nil {
		t.Errorf("Stat() error = %v, want the directory to remain", err)
	}
}

func TestRMRecursive(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/dir/sub/a", []byte("a"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	for _, flags := range []string{"-r", "-R", "-rf"} {
		t.Run(flags, func(t *testing.T) {
			if err := core.FS().MkdirAll("/srv/tree/sub", 0o755); err != nil {
				t.Fatalf("MkdirAll() error: %v", err)
			}

			code, err := core.RunScript(context.Background(), "rm "+flags+" /srv/tree", nil, nil, nil)
			if err != nil {
				t.Fatalf("RunScript() error: %v", err)
			}

			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}

			if _, err := core.FS().Lstat("/srv/tree"); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("Lstat() error = %v, want ErrNotExist", err)
			}
		})
	}
}

func TestRMMissing(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "rm /srv/missing", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	code, err = core.RunScript(context.Background(), "rm -f /srv/missing", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("rm -f exit code = %d, want 0", code)
	}

	code, err = core.RunScript(context.Background(), "rm -r /srv/missing", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("rm -r exit code = %d, want 1", code)
	}

	code, err = core.RunScript(context.Background(), "rm -rf /srv/missing", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("rm -rf exit code = %d, want 0", code)
	}
}

func TestRMMissingOperand(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "rm", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	code, err = core.RunScript(context.Background(), "rm -f", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("rm -f exit code = %d, want 0", code)
	}
}

func TestRMMultiple(t *testing.T) {
	core := New()

	for _, p := range []string{"/srv/a", "/srv/b"} {
		if err := core.FS().WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", p, err)
		}
	}

	code, err := core.RunScript(context.Background(), "rm /srv/a /srv/missing /srv/b", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	for _, p := range []string{"/srv/a", "/srv/b"} {
		if _, err := core.FS().Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Lstat(%s) error = %v, want ErrNotExist", p, err)
		}
	}
}

func TestRMOptionTerminator(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/-f", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "rm -- /srv/-f", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Lstat("/srv/-f"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}
}

func TestRMInvalidOption(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "rm -q /srv/a", nil, nil, &stderr)
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

func TestRMRelative(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "cd /srv && rm a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Lstat("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}
}

func TestRMNestedSh(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "sh -c 'rm /srv/a'", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Lstat("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}
}

func TestRMSudo(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "sudo rm /srv/a", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("rm", "/srv/a")
	if !ok {
		t.Fatal("rm command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}

func TestRMSymlink(t *testing.T) {
	core := New()

	if err := core.FS().WriteFile("/srv/target", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().Symlink("/srv/target", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	code, err := core.RunScript(context.Background(), "rm /srv/link", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if _, err := core.FS().Lstat("/srv/link"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(link) error = %v, want ErrNotExist", err)
	}
	if _, err := core.FS().Stat("/srv/target"); err != nil {
		t.Errorf("Stat(target) error = %v, want target to remain", err)
	}
}

func TestFSRemove(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := fsys.Remove("/srv/a"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := fsys.Lstat("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}

	if err := fsys.Remove("/srv/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Remove(missing) error = %v, want ErrNotExist", err)
	}

	if err := fsys.MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Remove("/srv/dir"); err != nil {
		t.Errorf("Remove(empty dir) error: %v", err)
	}

	if err := fsys.MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.WriteFile("/srv/dir/a", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Remove("/srv/dir"); !errors.Is(err, errDirNotEmpty) {
		t.Errorf("Remove(non-empty dir) error = %v, want errDirNotEmpty", err)
	}

	if err := fsys.Remove("/"); !errors.Is(err, errRemoveRoot) {
		t.Errorf("Remove(root) error = %v, want errRemoveRoot", err)
	}
}

func TestFSRemoveAll(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/dir/sub/a", []byte("a"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := fsys.RemoveAll("/srv/dir"); err != nil {
		t.Fatalf("RemoveAll() error: %v", err)
	}

	if _, err := fsys.Lstat("/srv/dir"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want ErrNotExist", err)
	}

	if err := fsys.RemoveAll("/srv/missing"); err != nil {
		t.Errorf("RemoveAll(missing) error = %v, want nil", err)
	}

	if err := fsys.RemoveAll("/"); !errors.Is(err, errRemoveRoot) {
		t.Errorf("RemoveAll(root) error = %v, want errRemoveRoot", err)
	}
}
