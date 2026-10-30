package gabric

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/bancek/gabric/internal/shelltest"
	"mvdan.cc/sh/v3/interp"
)

func TestRemoteStat(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	path := "/srv/file.txt"
	if err := core.FS().WriteFile(path, []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().SetOwner(path, "testuser", "staff"); err != nil {
		t.Fatalf("SetOwner() error: %v", err)
	}

	stat, err := Remote(ctx).Stat(path).RunErr()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
		t.Errorf("Mode.Perm() = %v, want %v", got, want)
	}
	if stat.Mode.IsDir() {
		t.Error("Mode.IsDir() = true, want false")
	}
	if stat.User != "testuser" || stat.Group != "staff" {
		t.Errorf("owner = %q:%q, want testuser:staff", stat.User, stat.Group)
	}

	asserts.AssertExecuted("uname", "-s")
	asserts.AssertExecuted("stat", "-L", "-c", "%f:%U:%G", path)
}

func TestRemoteStatPathWithSpaces(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	path := "/srv/a b.txt"
	if err := core.FS().WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	stat, err := Remote(ctx).Stat(path).RunErr()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got, want := stat.Mode.Perm(), fs.FileMode(0o644); got != want {
		t.Errorf("Mode.Perm() = %v, want %v", got, want)
	}

	asserts.AssertExecuted("stat", "-L", "-c", "%f:%U:%G", path)
}

func TestRemoteStatMissing(t *testing.T) {
	_, _, ctx := localRemoteContext(t)

	path := "/srv/missing.txt"

	if _, err := Remote(ctx).Stat(path).RunErr(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(missing) error = %v, want fs.ErrNotExist", err)
	}
}

func TestRemoteStatSymlink(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	path := "/srv/links/file.txt"
	dir := "/srv/links/dir"
	link := "/srv/links/file-link"
	dirLink := "/srv/links/dir-link"
	dangling := "/srv/links/dangling"

	if err := core.FS().WriteFile(path, []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().SetOwner(path, "testuser", "staff"); err != nil {
		t.Fatalf("SetOwner() error: %v", err)
	}
	if err := core.FS().MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().Symlink(path, link); err != nil {
		t.Fatalf("Symlink(file) error: %v", err)
	}
	if err := core.FS().Symlink(dir, dirLink); err != nil {
		t.Fatalf("Symlink(dir) error: %v", err)
	}
	if err := core.FS().Symlink("/srv/links/missing", dangling); err != nil {
		t.Fatalf("Symlink(dangling) error: %v", err)
	}

	stat, err := Remote(ctx).Stat(link).RunErr()
	if err != nil {
		t.Fatalf("Stat(link) error: %v", err)
	}
	if !stat.Mode.IsRegular() {
		t.Errorf("Mode = %v, want a regular file", stat.Mode)
	}
	if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
		t.Errorf("Mode.Perm() = %v, want %v", got, want)
	}
	if stat.User != "testuser" || stat.Group != "staff" {
		t.Errorf("owner = %q:%q, want testuser:staff", stat.User, stat.Group)
	}

	asserts.AssertExecuted("stat", "-L", "-c", "%f:%U:%G", link)

	dirStat, err := Remote(ctx).Stat(dirLink).RunErr()
	if err != nil {
		t.Fatalf("Stat(dirLink) error: %v", err)
	}
	if !dirStat.Mode.IsDir() {
		t.Error("Mode.IsDir() = false, want true")
	}

	if _, err := Remote(ctx).Stat(dangling).RunErr(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(dangling) error = %v, want fs.ErrNotExist", err)
	}
}

func TestRemoteStatError(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	path := "/srv/file.txt"
	if err := core.FS().WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	core.Register("stat", shelltest.Handler{
		Run: func(context.Context, shelltest.Command) error {
			return interp.ExitStatus(1)
		},
	})

	if _, err := Remote(ctx).Stat(path).RunErr(); err == nil {
		t.Fatal("Stat() error = nil, want error")
	} else if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want a non-missing error", err)
	}
}

func TestParseFileStat(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantMode  fs.FileMode
		wantUser  string
		wantGroup string
	}{
		{"gnu file", "gnu:81a0:testuser:staff\n", 0o640, "testuser", "staff"},
		{"gnu dir", "gnu:41ed:root:root\n", fs.ModeDir | 0o755, "root", "root"},
		{"bsd file", "bsd:100640:testuser:staff\n", 0o640, "testuser", "staff"},
		{"bsd dir", "bsd:41777:root:wheel\n", fs.ModeDir | fs.ModeSticky | 0o777, "root", "wheel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat, err := parseFileStat(tt.in)
			if err != nil {
				t.Fatalf("parseFileStat() error: %v", err)
			}

			if got := stat.Mode.Perm(); got != tt.wantMode.Perm() {
				t.Errorf("Mode.Perm() = %v, want %v", got, tt.wantMode.Perm())
			}
			if got, want := stat.Mode.IsDir(), tt.wantMode.IsDir(); got != want {
				t.Errorf("Mode.IsDir() = %v, want %v", got, want)
			}
			if got, want := stat.Mode&fs.ModeSticky != 0, tt.wantMode&fs.ModeSticky != 0; got != want {
				t.Errorf("Mode sticky = %v, want %v", got, want)
			}
			if stat.User != tt.wantUser || stat.Group != tt.wantGroup {
				t.Errorf("owner = %q:%q, want %q:%q", stat.User, stat.Group, tt.wantUser, tt.wantGroup)
			}
		})
	}
}

func TestParseFileStatMissing(t *testing.T) {
	if _, err := parseFileStat("missing:\n"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("parseFileStat(missing) error = %v, want fs.ErrNotExist", err)
	}
}

func TestParseFileStatErrors(t *testing.T) {
	tests := []string{
		"",
		"100640:root:root",
		"gnu:zz:root:root",
		"gnu:81a0:root",
	}

	for _, in := range tests {
		if _, err := parseFileStat(in); err == nil {
			t.Errorf("parseFileStat(%q) error = nil, want error", in)
		}
	}
}

func TestUnixFileMode(t *testing.T) {
	tests := []struct {
		name string
		raw  uint32
		want fs.FileMode
	}{
		{"regular", 0o100644, 0o644},
		{"dir", 0o040755, fs.ModeDir | 0o755},
		{"setuid dir", 0o044755, fs.ModeDir | fs.ModeSetuid | 0o755},
		{"setgid", 0o102755, fs.ModeSetgid | 0o755},
		{"sticky dir", 0o041777, fs.ModeDir | fs.ModeSticky | 0o777},
		{"symlink", 0o120777, fs.ModeSymlink | 0o777},
		{"fifo", 0o010644, fs.ModeNamedPipe | 0o644},
		{"socket", 0o140644, fs.ModeSocket | 0o644},
		{"device", 0o060644, fs.ModeDevice | 0o644},
		{"char device", 0o020644, fs.ModeDevice | fs.ModeCharDevice | 0o644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unixFileMode(tt.raw); got != tt.want {
				t.Errorf("unixFileMode(%o) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
