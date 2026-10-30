//go:build unix

package gabric

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStat(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	sub := filepath.Join(dir, "sub")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(file, 0o640); err != nil {
		t.Fatalf("Chmod(file) error: %v", err)
	}
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.Chmod(sub, 0o750); err != nil {
		t.Fatalf("Chmod(sub) error: %v", err)
	}
	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		stat, err := Local(ctx).Stat("file.txt").RunErr()
		if err != nil {
			t.Fatalf("Stat(file.txt) error: %v", err)
		}
		if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
			t.Errorf("file mode = %v, want %v", got, want)
		}
		if stat.Mode.IsDir() {
			t.Error("file Mode.IsDir() = true, want false")
		}
		if stat.User == "" || stat.Group == "" {
			t.Errorf("owner = %q:%q, want names", stat.User, stat.Group)
		}

		dirStat, err := Local(ctx).Stat("sub").RunErr()
		if err != nil {
			t.Fatalf("Stat(sub) error: %v", err)
		}
		if !dirStat.Mode.IsDir() {
			t.Error("sub Mode.IsDir() = false, want true")
		}
		if got, want := dirStat.Mode.Perm(), fs.FileMode(0o750); got != want {
			t.Errorf("dir mode = %v, want %v", got, want)
		}

		linkStat, err := Local(ctx).Stat("link").RunErr()
		if err != nil {
			t.Fatalf("Stat(link) error: %v", err)
		}
		if !linkStat.Mode.IsRegular() {
			t.Errorf("link mode = %v, want a regular file target", linkStat.Mode)
		}
		if got, want := linkStat.Mode.Perm(), fs.FileMode(0o640); got != want {
			t.Errorf("link target mode = %v, want %v", got, want)
		}
	})
}

func TestLocalStatMissing(t *testing.T) {
	ctx := testContext(t)
	path := filepath.Join(t.TempDir(), "missing")

	if _, err := Local(ctx).Stat(path).RunErr(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(missing) error = %v, want fs.ErrNotExist", err)
	}
}
