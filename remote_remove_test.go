package gabric

import (
	"errors"
	"io"
	"io/fs"
	"slices"
	"sync"
	"testing"
)

func removeRecordingHandler(mu *sync.Mutex, cmds *[]string) localExecHandler {
	return func(cmd string, _ io.Reader, _, _ io.Writer) int {
		mu.Lock()
		*cmds = append(*cmds, cmd)
		mu.Unlock()

		return 0
	}
}

func TestRemoteRemoveCommands(t *testing.T) {
	tests := []struct {
		name string
		ops  func(*RemoveOps) *RemoveOps
		want string
	}{
		{
			name: "file",
			ops:  func(ops *RemoveOps) *RemoveOps { return ops },
			want: "rm -- /srv/app/target",
		},
		{
			name: "force",
			ops:  func(ops *RemoveOps) *RemoveOps { return ops.Force() },
			want: "rm -f -- /srv/app/target",
		},
		{
			name: "recursive",
			ops:  func(ops *RemoveOps) *RemoveOps { return ops.Recursive() },
			want: "rm -r -- /srv/app/target",
		},
		{
			name: "recursive force",
			ops:  func(ops *RemoveOps) *RemoveOps { return ops.Recursive().Force() },
			want: "rm -rf -- /srv/app/target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mu   sync.Mutex
				cmds []string
			)

			_, _, ctx := localRemoteContextWithExec(t, removeRecordingHandler(&mu, &cmds), false)

			if err := tt.ops(Remote(ctx).Remove("/srv/app/target")).RunErr(); err != nil {
				t.Fatalf("RunErr() error: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()

			want := []string{"/bin/sh -c '" + tt.want + "'"}
			if !slices.Equal(cmds, want) {
				t.Errorf("commands = %#v, want %#v", cmds, want)
			}
		})
	}
}

func TestRemoteRemoveShellFile(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	target := "/srv/app/file.txt"

	if err := core.FS().WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Remote(ctx).Remove(target).RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := core.FS().Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}

	asserts.AssertExecuted("rm", "--", target)
}

func TestRemoteRemoveShellNonEmptyDir(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	target := "/srv/app/tree"

	if err := core.FS().MkdirAll(target+"/sub", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile(target+"/sub/file", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	err := Remote(ctx).Remove(target).RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(non-empty dir) = nil, want error")
	}

	if _, err := core.FS().Stat(target); err != nil {
		t.Errorf("Stat() error = %v, want the directory to remain", err)
	}

	if err := Remote(ctx).Remove(target).Recursive().RunErr(); err != nil {
		t.Fatalf("Remove().Recursive() error: %v", err)
	}

	if _, err := core.FS().Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat() error = %v, want fs.ErrNotExist", err)
	}

	asserts.AssertExecuted("rm", "-r", "--", target)
}

func TestRemoteRemoveShellMissing(t *testing.T) {
	_, asserts, ctx := localRemoteContext(t)

	missing := "/srv/app/missing"

	err := Remote(ctx).Remove(missing).RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(missing) = nil, want error")
	}

	if err := Remote(ctx).Remove(missing).Force().RunErr(); err != nil {
		t.Errorf("Remove(missing).Force() error = %v, want nil", err)
	}

	err = Remote(ctx).Remove(missing).Recursive().RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(missing).Recursive() = nil, want error")
	}

	if err := Remote(ctx).Remove(missing).Recursive().Force().RunErr(); err != nil {
		t.Errorf("Remove(missing).Recursive().Force() error = %v, want nil", err)
	}

	asserts.AssertExecuted("rm", "-rf", "--", missing)
}

func TestRemoteRemoveShellSymlink(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	target := "/srv/app/target.txt"
	link := "/srv/app/link.txt"

	if err := core.FS().WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := core.FS().Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := Remote(ctx).Remove(link).RunErr(); err != nil {
		t.Fatalf("Remove(link) error: %v", err)
	}

	if _, err := core.FS().Lstat(link); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(link) error = %v, want fs.ErrNotExist", err)
	}
	if _, err := core.FS().Stat(target); err != nil {
		t.Errorf("Stat(target) error = %v, want target to remain", err)
	}
}

func TestRemoteRemoveWorkingDir(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	target := "/srv/app/file.txt"

	if err := core.FS().WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Remote(ctx).Remove("file.txt").RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	asserts.AssertExecuted("rm", "--", target)
}

func TestRemoteRemoveSudo(t *testing.T) {
	core, asserts, ctx := localRemoteContextWithExec(t, nil, true)

	target := "/srv/app/file.txt"

	if err := core.FS().WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Remote(ctx).Remove(target).RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	cmd, ok := asserts.Find("rm", "--", target)
	if !ok {
		t.Fatal("rm command not recorded")
	}
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}
