//go:build unix

package gabric

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPut(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		Local(ctx).Put("file.txt").String("hello").Run()
	})

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}
}

func TestLocalPutRunErr(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if err := Local(ctx).Put("file.txt").String("hello").RunErr(); err != nil {
			t.Fatalf("RunErr() error: %v", err)
		}
	})

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}
}

func TestLocalPutEnsureParent(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "file.txt")

	if err := Local(ctx).Put(path).EnsureParent().String("hello").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}
}

func TestLocalPutMissingDir(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if err := Local(ctx).Put("missing/file.txt").String("x").RunErr(); err == nil {
			t.Error("RunErr(missing dir) = nil, want error")
		}
	})
}

func TestLocalPutMode(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := Local(ctx).Put(path).String("x").Mode(0o600).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want %v", got, os.FileMode(0o600))
	}
}

func TestLocalPutUserUnknown(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	err := Local(ctx).Put(filepath.Join(dir, "file.txt")).
		String("x").
		User("gabric-no-such-user").
		RunErr()
	if err != nil {
		return
	}

	t.Error("RunErr(unknown user) = nil, want error")
}

func TestLocalPutNoContent(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	if err := Local(ctx).Put(filepath.Join(dir, "file.txt")).RunErr(); err == nil {
		t.Error("RunErr(no content) = nil, want error")
	}
}

func TestLocalPutRunPanics(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	Local(ctx).Put(filepath.Join(dir, "missing", "file.txt")).String("x").Run()
}
