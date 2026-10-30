//go:build unix

package gabric

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalExists(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	sub := filepath.Join(dir, "sub")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if !Local(ctx).Exists("file.txt").Run() {
			t.Error("Exists(file.txt) = false, want true")
		}

		if Local(ctx).Exists("missing.txt").Run() {
			t.Error("Exists(missing.txt) = true, want false")
		}

		if got := Local(ctx).Exists("sub").Dir().Run(); !got {
			t.Error("Exists(sub).Dir() = false, want true")
		}

		if got := Local(ctx).Exists("file.txt").Dir().Run(); got {
			t.Error("Exists(file.txt).Dir() = true, want false")
		}

		if got := Local(ctx).Exists("file.txt").File().Run(); !got {
			t.Error("Exists(file.txt).File() = false, want true")
		}

		if got := Local(ctx).Exists("sub").File().Run(); got {
			t.Error("Exists(sub).File() = true, want false")
		}

		if got := Local(ctx).Exists("link").Link().Run(); !got {
			t.Error("Exists(link).Link() = false, want true")
		}

		if got := Local(ctx).Exists("file.txt").Link().Run(); got {
			t.Error("Exists(file.txt).Link() = true, want false")
		}

		// Dir/File follow symlinks (os.Stat), so a symlink to a directory
		// matches Dir and a symlink to a file matches File.
		if got := Local(ctx).Exists("link").File().Run(); !got {
			t.Error("Exists(link).File() = false, want true")
		}
	})
}

func TestLocalExistsRunErr(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		exists, err := Local(ctx).Exists("file.txt").RunErr()
		if err != nil {
			t.Fatalf("RunErr(file.txt) error: %v", err)
		}
		if !exists {
			t.Error("RunErr(file.txt) = false, want true")
		}

		exists, err = Local(ctx).Exists("missing.txt").RunErr()
		if err != nil {
			t.Fatalf("RunErr(missing.txt) error: %v", err)
		}
		if exists {
			t.Error("RunErr(missing.txt) = true, want false")
		}
	})
}
