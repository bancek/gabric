//go:build unix

package shelltest

import (
	"errors"
	"io/fs"
	"testing"
)

func TestFSWriteRead(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/app/config", []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := fsys.ReadFile("/srv/app/config")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(got) != "hello" {
		t.Errorf("ReadFile() = %q, want %q", got, "hello")
	}

	info, err := fsys.Stat("/srv/app/config")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if info.Name() != "config" {
		t.Errorf("Name() = %q, want %q", info.Name(), "config")
	}
	if info.Size() != 5 {
		t.Errorf("Size() = %d, want 5", info.Size())
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("Mode() = %v, want %v", got, want)
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}

	parent, err := fsys.Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat(parent) error: %v", err)
	}
	if !parent.IsDir() {
		t.Error("parent IsDir() = false, want true")
	}

	if _, err := fsys.ReadFile("/srv/app/missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile(missing) error = %v, want ErrNotExist", err)
	}
}

func TestFSMkdir(t *testing.T) {
	fsys := NewFS()

	if err := fsys.Mkdir("/missing/app", 0o755); err == nil {
		t.Error("Mkdir() = nil, want missing parent error")
	}

	if err := fsys.MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if err := fsys.Mkdir("/srv/app", 0o700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	info, err := fsys.Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if !info.IsDir() {
		t.Error("IsDir() = false, want true")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	if err := fsys.Mkdir("/srv/app", 0o755); err == nil {
		t.Error("Mkdir(existing) = nil, want error")
	}
}

func TestFSChmod(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/app/config", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := fsys.Chmod("/srv/app/config", 0o600); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	info, err := fsys.Stat("/srv/app/config")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	if err := fsys.Chmod("/srv/missing", 0o600); err == nil {
		t.Error("Chmod(missing) = nil, want error")
	}
}

func TestFSMkdirReadDir(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv/app", 0o750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if err := fsys.WriteFile("/srv/app/b", []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.WriteFile("/srv/app/a", []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	entries, err := fsys.ReadDir("/srv/app")
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Name() != "a" || entries[1].Name() != "b" {
		t.Errorf("entries = [%s %s], want [a b]", entries[0].Name(), entries[1].Name())
	}

	info, err := fsys.Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o750); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	if _, err := fsys.ReadDir("/srv/app/a"); err == nil {
		t.Error("ReadDir(file) error = nil, want error")
	}
}

func TestFSResolve(t *testing.T) {
	fsys := NewFS()

	tests := []struct {
		cwd  string
		name string
		want string
	}{
		{"/", "/", "/"},
		{"", "x", "/x"},
		{"/srv/app", "config", "/srv/app/config"},
		{"/srv/app", "../other", "/srv/other"},
		{"/srv/app", "../../etc/passwd", "/etc/passwd"},
		{"/srv/app", "/absolute", "/absolute"},
	}

	for _, tt := range tests {
		got, err := fsys.Resolve(tt.cwd, tt.name)
		if err != nil {
			t.Errorf("Resolve(%q, %q) error: %v", tt.cwd, tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tt.cwd, tt.name, got, tt.want)
		}
	}

	if _, err := fsys.Resolve("relative", "x"); err == nil {
		t.Error("Resolve(relative cwd) error = nil, want error")
	}
}

func TestFSEscape(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/../../x", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := fsys.ReadFile("/x")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile() = %q, want %q", got, "x")
	}
}

func TestFSHostIsolation(t *testing.T) {
	fsys := NewFS()

	if _, err := fsys.Stat("/etc/passwd"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(/etc/passwd) error = %v, want ErrNotExist", err)
	}
}

func TestFSOverwrite(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/x", []byte("one"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.WriteFile("/x", []byte("two"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := fsys.ReadFile("/x")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != "two" {
		t.Errorf("ReadFile() = %q, want %q", got, "two")
	}

	info, err := fsys.Stat("/x")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestFSMkdirOnFile(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := fsys.MkdirAll("/file", 0o755); err == nil {
		t.Error("MkdirAll(file) error = nil, want error")
	}
}

func TestFSSymlinkStat(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/app/file", []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Symlink("/srv/app/file", "/srv/app/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	linkInfo, err := fsys.Lstat("/srv/app/link")
	if err != nil {
		t.Fatalf("Lstat() error: %v", err)
	}
	if linkInfo.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Mode() = %v, want ModeSymlink", linkInfo.Mode())
	}
	if got, want := linkInfo.Name(), "link"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if linkInfo.IsDir() {
		t.Error("IsDir() = true, want false")
	}

	info, err := fsys.Stat("/srv/app/link")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("Mode() = %v, want a regular file", info.Mode())
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o640); got != want {
		t.Errorf("Mode().Perm() = %v, want %v", got, want)
	}
	if got, want := info.Name(), "link"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	got, err := fsys.ReadFile("/srv/app/link")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile() = %q, want %q", got, "x")
	}

	entries, err := fsys.ReadDir("/srv/app")
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[1].Name() != "link" {
		t.Errorf("entry name = %q, want %q", entries[1].Name(), "link")
	}
	if entries[1].Type()&fs.ModeSymlink == 0 {
		t.Errorf("entry Type() = %v, want ModeSymlink", entries[1].Type())
	}
	if entries[1].IsDir() {
		t.Error("entry IsDir() = true, want false")
	}
}

func TestFSSymlinkToDir(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv/app", 0o750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("/srv/app", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	info, err := fsys.Stat("/srv/link")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if !info.IsDir() {
		t.Error("IsDir() = false, want true")
	}

	if _, err := fsys.ReadDir("/srv/link"); err != nil {
		t.Fatalf("ReadDir(link) error: %v", err)
	}

	if err := fsys.WriteFile("/srv/link/sub/file", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(link/sub/file) error: %v", err)
	}

	got, err := fsys.ReadFile("/srv/app/sub/file")
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile(target) = %q, want %q", got, "x")
	}

	if err := fsys.MkdirAll("/srv/link/another", 0o755); err != nil {
		t.Fatalf("MkdirAll(link/another) error: %v", err)
	}

	if info, err := fsys.Stat("/srv/app/another"); err != nil {
		t.Fatalf("Stat(target) error: %v", err)
	} else if !info.IsDir() {
		t.Error("target IsDir() = false, want true")
	}
}

func TestFSSymlinkRelativeTarget(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/app/file", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.MkdirAll("/srv/app/links", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("../file", "/srv/app/links/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	got, err := fsys.ReadFile("/srv/app/links/link")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile() = %q, want %q", got, "x")
	}
}

func TestFSSymlinkDangling(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("/srv/missing", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if info, err := fsys.Lstat("/srv/link"); err != nil {
		t.Fatalf("Lstat() error: %v", err)
	} else if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat Mode() = %v, want ModeSymlink", info.Mode())
	}

	if _, err := fsys.Stat("/srv/link"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want ErrNotExist", err)
	}
	if _, err := fsys.ReadFile("/srv/link"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile() error = %v, want ErrNotExist", err)
	}

	if err := fsys.WriteFile("/srv/link", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(link) error: %v", err)
	}

	got, err := fsys.ReadFile("/srv/missing")
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile(target) = %q, want %q", got, "x")
	}

	if info, err := fsys.Lstat("/srv/link"); err != nil {
		t.Fatalf("Lstat() error: %v", err)
	} else if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat Mode() = %v, want ModeSymlink", info.Mode())
	}
}

func TestFSSymlinkLoop(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("/srv/b", "/srv/a"); err != nil {
		t.Fatalf("Symlink(a) error: %v", err)
	}
	if err := fsys.Symlink("/srv/a", "/srv/b"); err != nil {
		t.Fatalf("Symlink(b) error: %v", err)
	}

	if _, err := fsys.Stat("/srv/a"); !errors.Is(err, errSymlinkLoop) {
		t.Errorf("Stat() error = %v, want errSymlinkLoop", err)
	}
}

func TestFSSymlinkChmod(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/file", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Symlink("/srv/file", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := fsys.Chmod("/srv/link", 0o600); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	info, err := fsys.Stat("/srv/file")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("Mode().Perm() = %v, want %v", got, want)
	}

	if linkInfo, err := fsys.Lstat("/srv/link"); err != nil {
		t.Fatalf("Lstat() error: %v", err)
	} else if linkInfo.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("link Mode() = %v, want ModeSymlink", linkInfo.Mode())
	}
}

func TestFSSymlinkRename(t *testing.T) {
	fsys := NewFS()

	if err := fsys.WriteFile("/srv/file", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := fsys.Symlink("/srv/file", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := fsys.Rename("/srv/link", "/srv/moved"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	if _, err := fsys.Lstat("/srv/link"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(old) error = %v, want ErrNotExist", err)
	}

	if info, err := fsys.Lstat("/srv/moved"); err != nil {
		t.Fatalf("Lstat(moved) error: %v", err)
	} else if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("moved Mode() = %v, want ModeSymlink", info.Mode())
	}

	got, err := fsys.ReadFile("/srv/file")
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("ReadFile(target) = %q, want %q", got, "x")
	}
}

func TestFSSymlinkMkdir(t *testing.T) {
	fsys := NewFS()

	if err := fsys.MkdirAll("/srv/dir", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := fsys.Symlink("/srv/dir", "/srv/link"); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := fsys.Mkdir("/srv/link", 0o755); !errors.Is(err, fs.ErrExist) {
		t.Errorf("Mkdir(link) error = %v, want ErrExist", err)
	}
	if err := fsys.MkdirAll("/srv/link", 0o755); err != nil {
		t.Errorf("MkdirAll(link) error = %v, want nil", err)
	}
	if err := fsys.MkdirAll("/srv/link/sub", 0o755); err != nil {
		t.Fatalf("MkdirAll(link/sub) error: %v", err)
	}
	if info, err := fsys.Stat("/srv/dir/sub"); err != nil {
		t.Fatalf("Stat(target) error: %v", err)
	} else if !info.IsDir() {
		t.Error("target IsDir() = false, want true")
	}

	if err := fsys.Symlink("/srv/missing", "/srv/dangling"); err != nil {
		t.Fatalf("Symlink(dangling) error: %v", err)
	}
	if err := fsys.Mkdir("/srv/dangling", 0o755); !errors.Is(err, fs.ErrExist) {
		t.Errorf("Mkdir(dangling) error = %v, want ErrExist", err)
	}
	if err := fsys.MkdirAll("/srv/dangling", 0o755); !errors.Is(err, fs.ErrExist) {
		t.Errorf("MkdirAll(dangling) error = %v, want ErrExist", err)
	}
}

func TestCoreFS(t *testing.T) {
	if New().FS() == nil {
		t.Error("FS() = nil, want filesystem")
	}
}
