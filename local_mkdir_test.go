//go:build unix

package gabric

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalMkdir(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b")

	if err := Local(ctx).Mkdir(target).RunErr(); err == nil {
		t.Error("Mkdir(nested) without Recursive = nil, want error")
	}

	if err := Local(ctx).Mkdir(target).Recursive().RunErr(); err != nil {
		t.Fatalf("Mkdir().Recursive() error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if !info.IsDir() {
		t.Error("target is not a directory")
	}
}

func TestLocalMkdirMode(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b")

	if err := Local(ctx).Mkdir(target).Recursive().Mode(0o700).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("mode = %v, want %v", got, os.FileMode(0o700))
	}

	if err := Local(ctx).Mkdir(target).Mode(0o755).RunErr(); err != nil {
		t.Fatalf("RunErr(existing) error: %v", err)
	}

	info, err = os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("mode after re-mkdir = %v, want %v", got, os.FileMode(0o755))
	}
}

func TestLocalMkdirFile(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "file")

	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Local(ctx).Mkdir(target).RunErr(); err == nil {
		t.Error("Mkdir(existing file) = nil, want error")
	}
}

func TestLocalMkdirUserUnknown(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		err := Local(ctx).Mkdir("sub").User("gabric-no-such-user").RunErr()
		if err != nil {
			return
		}

		t.Error("RunErr(unknown user) = nil, want error")
	})
}

func TestLocalMkdirRunPanics(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	Local(ctx).Mkdir(filepath.Join(dir, "a", "b")).Run()
}
