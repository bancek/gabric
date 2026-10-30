package gabric

import (
	"io"
	"slices"
	"sync"
	"testing"
)

func TestRemoteExists(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	dir := "/srv"
	file := "/srv/file.txt"
	sub := "/srv/sub"

	if err := core.FS().WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := core.FS().MkdirAll(sub, 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if !Remote(ctx).Exists(file).Run() {
		t.Error("Exists(file) = false, want true")
	}

	if Remote(ctx).Exists(dir + "/missing.txt").Run() {
		t.Error("Exists(missing) = true, want false")
	}

	if got := Remote(ctx).Exists(sub).Dir().Run(); !got {
		t.Error("Exists(sub).Dir() = false, want true")
	}

	if got := Remote(ctx).Exists(file).Dir().Run(); got {
		t.Error("Exists(file).Dir() = true, want false")
	}

	if got := Remote(ctx).Exists(file).File().Run(); !got {
		t.Error("Exists(file).File() = false, want true")
	}

	if got := Remote(ctx).Exists(sub).File().Run(); got {
		t.Error("Exists(sub).File() = true, want false")
	}
}

func TestRemoteExistsSymlink(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	file := "/srv/links/file.txt"
	dir := "/srv/links/dir"
	link := "/srv/links/file-link"
	dirLink := "/srv/links/dir-link"
	dangling := "/srv/links/dangling"

	if err := core.FS().WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().Symlink(file, link); err != nil {
		t.Fatalf("Symlink(file) error: %v", err)
	}
	if err := core.FS().Symlink(dir, dirLink); err != nil {
		t.Fatalf("Symlink(dir) error: %v", err)
	}
	if err := core.FS().Symlink("/srv/links/missing", dangling); err != nil {
		t.Fatalf("Symlink(dangling) error: %v", err)
	}

	if got := Remote(ctx).Exists(link).Link().Run(); !got {
		t.Error("Exists(link).Link() = false, want true")
	}

	if got := Remote(ctx).Exists(file).Link().Run(); got {
		t.Error("Exists(file).Link() = true, want false")
	}

	if got := Remote(ctx).Exists(link).File().Run(); !got {
		t.Error("Exists(link).File() = false, want true")
	}

	if got := Remote(ctx).Exists(link).Run(); !got {
		t.Error("Exists(link) = false, want true")
	}

	if got := Remote(ctx).Exists(link).Dir().Run(); got {
		t.Error("Exists(link).Dir() = true, want false")
	}

	if got := Remote(ctx).Exists(dirLink).Dir().Run(); !got {
		t.Error("Exists(dirLink).Dir() = false, want true")
	}

	if got := Remote(ctx).Exists(dirLink).File().Run(); got {
		t.Error("Exists(dirLink).File() = true, want false")
	}

	if got := Remote(ctx).Exists(dangling).Link().Run(); !got {
		t.Error("Exists(dangling).Link() = false, want true")
	}

	if got := Remote(ctx).Exists(dangling).Run(); got {
		t.Error("Exists(dangling) = true, want false")
	}

	if got := Remote(ctx).Exists(dangling).File().Run(); got {
		t.Error("Exists(dangling).File() = true, want false")
	}
}

// existsRecordingHandler records every command.
func existsRecordingHandler(mu *sync.Mutex, cmds *[]string) localExecHandler {
	return func(cmd string, _ io.Reader, _, _ io.Writer) int {
		mu.Lock()
		*cmds = append(*cmds, cmd)
		mu.Unlock()

		return 0
	}
}

func TestRemoteExistsCommands(t *testing.T) {
	var mu sync.Mutex
	var cmds []string

	_, _, ctx := localRemoteContextWithExec(t, existsRecordingHandler(&mu, &cmds), false)

	if got := Remote(ctx).Exists("/srv/dir").Dir().Run(); !got {
		t.Error("Exists(dir).Dir() = false, want true")
	}

	if got := Remote(ctx).Exists("/srv/file").File().Run(); !got {
		t.Error("Exists(file).File() = false, want true")
	}

	if got := Remote(ctx).Exists("/srv/link").Link().Run(); !got {
		t.Error("Exists(link).Link() = false, want true")
	}

	if got := Remote(ctx).Exists("/srv/any").Run(); !got {
		t.Error("Exists(any) = false, want true")
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{
		"/bin/sh -c 'test -d /srv/dir'",
		"/bin/sh -c 'test -f /srv/file'",
		"/bin/sh -c 'test -L /srv/link'",
		"/bin/sh -c 'test -e /srv/any'",
	}

	if !slices.Equal(cmds, want) {
		t.Errorf("commands = %#v, want %#v", cmds, want)
	}
}

func TestRemoteExistsRunErr(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	path := "/srv/file.txt"
	if err := core.FS().WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	exists, err := Remote(ctx).Exists(path).RunErr()
	if err != nil {
		t.Fatalf("RunErr(file.txt) error: %v", err)
	}
	if !exists {
		t.Error("RunErr(file.txt) = false, want true")
	}

	exists, err = Remote(ctx).Exists("/srv/missing.txt").RunErr()
	if err != nil {
		t.Fatalf("RunErr(missing.txt) error: %v", err)
	}
	if exists {
		t.Error("RunErr(missing.txt) = true, want false")
	}
}
