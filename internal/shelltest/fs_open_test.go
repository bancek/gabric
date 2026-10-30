//go:build unix

package shelltest

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
)

func TestFSOpenSymlink(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/file", []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Symlink("/srv/file", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	f, err := fsys.openAt("/", "/srv/link", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("openAt(read) error: %v", err)
	}

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if got, want := string(got), "hello"; got != want {
		t.Errorf("ReadAll() = %q, want %q", got, want)
	}

	w, err := fsys.openAt("/", "/srv/link", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("openAt(write) error: %v", err)
	}
	if _, err := w.Write([]byte("world")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	target, err := fsys.ReadFile("/srv/file")
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if got, want := string(target), "world"; got != want {
		t.Errorf("target = %q, want %q", got, want)
	}

	info, err := fsys.Lstat("/srv/link")
	if err != nil {
		t.Fatalf("Lstat() error: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("link Mode() = %v, want ModeSymlink", info.Mode())
	}
}

func TestFSOpenSymlinkExclusive(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/file", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Symlink("/srv/file", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if _, err := fsys.openAt("/", "/srv/link", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); !errors.Is(err, fs.ErrExist) {
		t.Errorf("openAt(exclusive) error = %v, want ErrExist", err)
	}
}

func TestFSOpenDanglingSymlink(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("/srv/missing", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	w, err := fsys.openAt("/", "/srv/link", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("openAt() error: %v", err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	got, err := fsys.ReadFile("/srv/missing")
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("target = %q, want %q", got, "x")
	}

	info, err := fsys.Lstat("/srv/link")
	if err != nil {
		t.Fatalf("Lstat() error: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("link Mode() = %v, want ModeSymlink", info.Mode())
	}
}
