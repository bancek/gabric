package gabric

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRemoveFile(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Local(ctx).Remove(target).RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLocalRemoveEmptyDir(t *testing.T) {
	ctx := testContext(t)
	target := filepath.Join(t.TempDir(), "empty")

	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	if err := Local(ctx).Remove(target).RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLocalRemoveNonEmptyDir(t *testing.T) {
	ctx := testContext(t)
	target := filepath.Join(t.TempDir(), "tree")

	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	err := Local(ctx).Remove(target).RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(non-empty dir) = nil, want error")
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("Stat() error = %v, want the directory to remain", err)
	}
}

func TestLocalRemoveRecursive(t *testing.T) {
	ctx := testContext(t)
	target := filepath.Join(t.TempDir(), "tree")

	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "file"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Local(ctx).Remove(target).Recursive().RunErr(); err != nil {
		t.Fatalf("Remove().Recursive() error: %v", err)
	}

	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLocalRemoveMissing(t *testing.T) {
	ctx := testContext(t)
	missing := filepath.Join(t.TempDir(), "missing")

	err := Local(ctx).Remove(missing).RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(missing) = nil, want error")
	}

	if err := Local(ctx).Remove(missing).Force().RunErr(); err != nil {
		t.Errorf("Remove(missing).Force() error = %v, want nil", err)
	}

	err = Local(ctx).Remove(missing).Recursive().RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(missing).Recursive() = nil, want error")
	}

	if err := Local(ctx).Remove(missing).Recursive().Force().RunErr(); err != nil {
		t.Errorf("Remove(missing).Recursive().Force() error = %v, want nil", err)
	}
}

func TestLocalRemoveSymlink(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("Symlink() error: %v", err)
	}

	if err := Local(ctx).Remove(link).RunErr(); err != nil {
		t.Fatalf("Remove(link) error: %v", err)
	}

	if _, err := os.Lstat(link); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(link) error = %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("Stat(target) error = %v, want target to remain", err)
	}
}

func TestLocalRemoveWorkingDir(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if err := Local(ctx).Remove("file.txt").RunErr(); err != nil {
			t.Errorf("Remove() error: %v", err)
		}
	})

	if _, err := os.Lstat(filepath.Join(dir, "file.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLocalRemoveRunPanics(t *testing.T) {
	ctx := testContext(t)

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	Local(ctx).Remove(filepath.Join(t.TempDir(), "missing")).Run()
}
