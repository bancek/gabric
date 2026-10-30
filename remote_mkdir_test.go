package gabric

import (
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

// mkdirRecordingHandler records every command except "test -d", which it
// answers as missing so the mkdir path is exercised.
func mkdirRecordingHandler(mu *sync.Mutex, cmds *[]string) localExecHandler {
	return func(cmd string, _ io.Reader, _, _ io.Writer) int {
		if strings.Contains(cmd, "test -d") {
			return 1
		}

		mu.Lock()
		*cmds = append(*cmds, cmd)
		mu.Unlock()

		return 0
	}
}

func TestRemoteMkdirCommands(t *testing.T) {
	var mu sync.Mutex
	var cmds []string

	_, _, ctx := localRemoteContextWithExec(t, mkdirRecordingHandler(&mu, &cmds), false)

	err := Remote(ctx).Mkdir("/srv/app/nested").
		Recursive().
		User("root").
		Group("root").
		Mode(0o700).
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{
		"/bin/sh -c 'mkdir -p /srv/app/nested'",
		"/bin/sh -c 'chown root:root /srv/app/nested'",
		"/bin/sh -c 'chmod 700 /srv/app/nested'",
	}

	if !slices.Equal(cmds, want) {
		t.Errorf("commands = %#v, want %#v", cmds, want)
	}
}

func TestRemoteMkdirNotRecursive(t *testing.T) {
	var mu sync.Mutex
	var cmds []string

	_, _, ctx := localRemoteContextWithExec(t, mkdirRecordingHandler(&mu, &cmds), false)

	if err := Remote(ctx).Mkdir("/srv/app/leaf").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{"/bin/sh -c 'mkdir /srv/app/leaf'"}

	if !slices.Equal(cmds, want) {
		t.Errorf("commands = %#v, want %#v", cmds, want)
	}
}

func TestRemoteMkdirShell(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	target := "/srv/app/nested"

	if err := Remote(ctx).Mkdir(target).Recursive().Mode(0o750).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	info, err := core.FS().Stat(target)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if !info.IsDir() {
		t.Error("target is not a directory")
	}

	if got := info.Mode().Perm(); got != 0o750 {
		t.Errorf("mode = %v, want %v", got, os.FileMode(0o750))
	}

	asserts.AssertExecuted("mkdir", "-p", target)
	asserts.AssertExecuted("chmod", "750", target)
}

func TestRemoteMkdirShellExisting(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	target := "/srv/app"
	if err := core.FS().MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if err := Remote(ctx).Mkdir(target).Mode(0o700).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	info, err := core.FS().Stat(target)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("mode = %v, want %v", got, os.FileMode(0o700))
	}

	asserts.AssertNotExecuted("mkdir", target)
	asserts.AssertExecuted("chmod", "700", target)
}

func TestRemoteMkdirShellNotRecursive(t *testing.T) {
	_, _, ctx := localRemoteContext(t)

	if err := Remote(ctx).Mkdir("/srv/app/nested").RunErr(); err == nil {
		t.Error("RunErr() error = nil, want missing parent error")
	}
}
