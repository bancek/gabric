//go:build unix

package gabric

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/bancek/gabric/internal/shelltest"
	"mvdan.cc/sh/v3/interp"
)

func TestLocalRemoteFileOps(t *testing.T) {
	dir := t.TempDir()
	ctx := NewRemoteWorkingDirContext(testContext(t), dir)
	ctx = NewTargetContext(ctx, LocalRemoteTarget)

	ops := TargetOps(ctx)

	if err := ops.Put("file.txt").String("hello").Mode(0o600).RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if got := ops.Get("file.txt").String(); got != "hello" {
		t.Errorf("Get() = %q, want %q", got, "hello")
	}

	if !ops.Exists("file.txt").Run() {
		t.Error("Exists(file.txt) = false, want true")
	}

	if ops.Exists("missing.txt").Run() {
		t.Error("Exists(missing.txt) = true, want false")
	}

	stat, err := ops.Stat("file.txt").RunErr()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := stat.Mode.Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("Stat().Mode.Perm() = %v, want %v", got, want)
	}

	if err := ops.Mkdir("sub").RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
}

func TestLocalRemoteRunnerExitError(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalRemoteTarget)

	err := TargetOps(ctx).Exec("sh", "-c", "exit 3").RunErr()

	if exitErr, ok := errors.AsType[*ExecExitError](err); !ok {
		t.Fatalf("error = %T, want *ExecExitError", err)
	} else if exitErr.Status != 3 {
		t.Errorf("Status = %d, want 3", exitErr.Status)
	}
}

func TestLocalRemoteTargetSudoFileOps(t *testing.T) {
	core, asserts, ctx := localRemoteTargetContext(t)

	core.Register("ls", shelltest.Handler{
		Run: func(ctx context.Context, _ shelltest.Command) error {
			_, err := io.WriteString(interp.HandlerCtx(ctx).Stdout, "a.txt\nsub/\n")
			return err
		},
	})

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	ops := TargetOps(ctx)

	if err := ops.Put("/srv/app/file.txt").String("hello").Mode(0o600).RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if got := ops.Get("/srv/app/file.txt").String(); got != "hello" {
		t.Errorf("Get() = %q, want hello", got)
	}
	if !ops.Exists("/srv/app/file.txt").File().Run() {
		t.Error("Exists(file) = false, want true")
	}
	if _, err := ops.Stat("/srv/app/file.txt").RunErr(); err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if err := ops.Mkdir("/srv/app/sub").RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if entries := ops.List("/srv/app"); len(entries) == 0 {
		t.Error("List() returned no entries")
	}

	// `test` is a shell builtin and is not recorded; check the coreutils
	// commands that back the file operations instead.
	for _, name := range []string{"dd", "cat", "stat", "mkdir", "chmod", "ls"} {
		cmd, ok := asserts.FindFunc(func(cmd shelltest.Command) bool {
			return cmd.Name == name
		})
		if !ok {
			t.Errorf("no %s command recorded", name)
			continue
		}
		if cmd.Sudo == nil {
			t.Errorf("%s ran without sudo metadata", name)
		}
	}
}
