package gabricremote2e

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrice2e"
	"github.com/bancek/gabric/internal/e2eimages"
)

func TestMain(m *testing.M) {
	gabrice2e.Main(m)
}

type e2eTarget struct {
	*gabrice2e.Test
	stop       func()
	sudo       bool
	workDir    string
	shell      func(script string) []string
	exit3      []string
	path       func(name string) string
	join       func(elem ...string) string
	posixModes bool
	symlinks   bool
}

func (g *e2eTarget) Stop() {
	g.stop()
}

func (g *e2eTarget) ops() gabric.Ops {
	return gabric.TargetOps(g.Context())
}

func (g *e2eTarget) shellExec(script string) *gabric.ExecOps {
	return g.ops().Exec(g.shell(script)...)
}

func (g *e2eTarget) targetPath(name string) string {
	return g.path(name)
}

var targets = []struct {
	name string
	new  func(t *testing.T) *e2eTarget
}{
	{name: "docker", new: newDockerTarget},
	{name: "ssh", new: newSSHTarget},
	{name: "localremote", new: newLocalRemoteTarget},
	{name: "local", new: newLocalTarget},
}

func newDockerTarget(t *testing.T) *e2eTarget {
	g := gabrice2e.Docker(t, e2eimages.DockerRsync)

	return &e2eTarget{
		Test:       g.Test,
		stop:       g.Container().Stop,
		sudo:       true,
		workDir:    "/tmp",
		shell:      unixShell,
		exit3:      unixExit3,
		path:       posixPath("/tmp"),
		join:       pathpkg.Join,
		posixModes: true,
		symlinks:   true,
	}
}

func newSSHTarget(t *testing.T) *e2eTarget {
	g := gabrice2e.SSH(t, e2eimages.DockerSSH, "gabric")

	return &e2eTarget{
		Test:       g.Test,
		stop:       g.Container().Stop,
		sudo:       true,
		workDir:    "/tmp",
		shell:      unixShell,
		exit3:      unixExit3,
		path:       posixPath("/tmp"),
		join:       pathpkg.Join,
		posixModes: true,
		symlinks:   true,
	}
}

func newLocalRemoteTarget(t *testing.T) *e2eTarget {
	g := gabrice2e.LocalRemote(t)
	workDir := gabric.GetRemoteWorkingDir(g.Context())

	return &e2eTarget{
		Test:       g,
		sudo:       false,
		workDir:    workDir,
		shell:      unixShell,
		exit3:      unixExit3,
		path:       localPath(workDir),
		join:       filepath.Join,
		posixModes: runtime.GOOS != "windows",
		symlinks:   true,
	}
}

func newLocalTarget(t *testing.T) *e2eTarget {
	g := gabrice2e.Local(t)
	workDir := gabric.GetLocalWorkingDir(g.Context())

	return &e2eTarget{
		Test:       g,
		sudo:       false,
		workDir:    workDir,
		shell:      shellForOS,
		exit3:      exit3ForOS(),
		path:       localPath(workDir),
		join:       filepath.Join,
		posixModes: runtime.GOOS != "windows",
		symlinks:   runtime.GOOS != "windows",
	}
}

func forEachTarget(t *testing.T, fn func(g *e2eTarget)) {
	t.Helper()

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			fn(tt.new(t))
		})
	}
}

func posixPath(base string) func(name string) string {
	return func(name string) string {
		return pathpkg.Join(base, uniqueName(name))
	}
}

func localPath(base string) func(name string) string {
	return func(name string) string {
		return filepath.Join(base, uniqueName(name))
	}
}

func uniqueName(name string) string {
	return "gabricremote2e-" + uuid.NewV4().String() + "-" + name
}

func unixShell(script string) []string {
	return []string{"sh", "-c", script}
}

var unixExit3 = []string{"sh", "-c", "exit 3"}

func shellForOS(script string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", script}
	}

	return unixShell(script)
}

func exit3ForOS() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "exit /b 3"}
	}

	return unixExit3
}

func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")

	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}

	return strings.Join(lines, "\n")
}

func exitStatus(err error) (int, bool) {
	if e, ok := errors.AsType[*gabric.ExecExitError](err); ok {
		return e.ExitCode(), true
	}

	return 0, false
}

func requireRsync(t testing.TB) {
	t.Helper()

	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
}

func rsyncOps(t testing.TB, ctx context.Context) *gabric.RsyncOps {
	t.Helper()

	ops, err := gabric.TargetRsyncOps(ctx)
	if errors.Is(err, gabric.ErrNoRsyncTarget) {
		t.Skip("target does not support rsync")
	}
	if err != nil {
		t.Fatalf("TargetRsyncOps() error: %v", err)
	}

	return ops
}

func TestExec(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		e := g.shellExec("echo out && echo err 1>&2").Capture()

		if err := e.RunErr(); err != nil {
			g.T().Fatalf("RunErr() error: %v", err)
		}
		if got, want := normalizeOutput(e.StdoutStr()), "out\n"; got != want {
			g.T().Errorf("stdout = %q, want %q", got, want)
		}
		if got, want := normalizeOutput(e.StderrStr()), "err\n"; got != want {
			g.T().Errorf("stderr = %q, want %q", got, want)
		}

		err := g.ops().Exec(g.exit3...).RunErr()

		status, ok := exitStatus(err)
		if !ok {
			g.T().Fatalf("error = %T, want an exit error", err)
		}
		if status != 3 {
			g.T().Errorf("exit status = %d, want 3", status)
		}
	})
}

func TestTargetOps(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		out := g.shellExec("echo target").
			CaptureStdout().
			Run().
			StdoutStr()

		if got, want := normalizeOutput(out), "target\n"; got != want {
			g.T().Errorf("stdout = %q, want %q", got, want)
		}
	})
}

func TestFiles(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		path := g.targetPath("file.txt")
		ops := g.ops()

		if err := ops.Put(path).String("hello").Mode(0o640).RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		if got, want := ops.Get(path).String(), "hello"; got != want {
			g.T().Errorf("Get() = %q, want %q", got, want)
		}

		exists, err := ops.Exists(path).RunErr()
		if err != nil {
			g.T().Fatalf("Exists().RunErr() error: %v", err)
		}
		if !exists {
			g.T().Error("Exists() = false, want true")
		}
	})
}

func TestStat(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		ops := g.ops()
		file := g.targetPath("stat.txt")

		if err := ops.Put(file).String("stat").Mode(0o640).RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		stat, err := ops.Stat(file).RunErr()
		if err != nil {
			g.T().Fatalf("Stat(file) error: %v", err)
		}
		if g.posixModes {
			if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
				g.T().Errorf("file mode = %v, want %v", got, want)
			}
		}
		if stat.Mode.IsDir() {
			g.T().Error("file Mode.IsDir() = true, want false")
		}

		dir := g.targetPath("stat-dir")

		if err := ops.Mkdir(dir).Mode(0o750).RunErr(); err != nil {
			g.T().Fatalf("Mkdir() error: %v", err)
		}

		dirStat, err := ops.Stat(dir).RunErr()
		if err != nil {
			g.T().Fatalf("Stat(dir) error: %v", err)
		}
		if !dirStat.Mode.IsDir() {
			g.T().Error("dir Mode.IsDir() = false, want true")
		}
		if g.posixModes {
			if got, want := dirStat.Mode.Perm(), fs.FileMode(0o750); got != want {
				g.T().Errorf("dir mode = %v, want %v", got, want)
			}
		}

		_, err = ops.Stat(g.targetPath("missing")).RunErr()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				g.T().Errorf("Stat(missing) error = %v, want fs.ErrNotExist", err)
			}
		} else {
			g.T().Error("Stat(missing) error = nil, want fs.ErrNotExist")
		}
	})
}

func TestList(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		ops := g.ops()
		dir := g.targetPath("list")

		if err := ops.Mkdir(dir).RunErr(); err != nil {
			g.T().Fatalf("Mkdir() error: %v", err)
		}
		if err := ops.Put(g.join(dir, "b.txt")).String("b").RunErr(); err != nil {
			g.T().Fatalf("Put(b.txt) error: %v", err)
		}
		if err := ops.Put(g.join(dir, ".hidden")).String("h").RunErr(); err != nil {
			g.T().Fatalf("Put(.hidden) error: %v", err)
		}
		if err := ops.Mkdir(g.join(dir, "sub")).RunErr(); err != nil {
			g.T().Fatalf("Mkdir(sub) error: %v", err)
		}

		entries := ops.List(dir)
		want := []gabric.ListEntry{
			{Name: ".hidden"},
			{Name: "b.txt"},
			{Name: "sub", IsDir: true},
		}

		if !slices.Equal(entries, want) {
			g.T().Errorf("List() = %+v, want %+v", entries, want)
		}
	})
}

func TestMkdir(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		ops := g.ops()
		dir := g.targetPath("mkdir")

		if err := ops.Mkdir(dir).Mode(0o750).RunErr(); err != nil {
			g.T().Fatalf("Mkdir() error: %v", err)
		}

		stat, err := ops.Stat(dir).RunErr()
		if err != nil {
			g.T().Fatalf("Stat() error: %v", err)
		}
		if !stat.Mode.IsDir() {
			g.T().Error("Mode.IsDir() = false, want true")
		}
	})
}

func TestRemove(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		ops := g.ops()

		file := g.targetPath("remove.txt")
		if err := ops.Put(file).String("x").RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		if err := ops.Remove(file).RunErr(); err != nil {
			g.T().Fatalf("Remove(file) error: %v", err)
		}

		exists, err := ops.Exists(file).RunErr()
		if err != nil {
			g.T().Fatalf("Exists() error: %v", err)
		}
		if exists {
			g.T().Error("file still exists after Remove")
		}

		err = ops.Remove(file).RunErr()
		if err != nil {
			// expected
		} else {
			g.T().Error("Remove(missing) = nil, want error")
		}

		if err := ops.Remove(file).Force().RunErr(); err != nil {
			g.T().Errorf("Remove(missing).Force() error: %v", err)
		}

		dir := g.targetPath("remove-dir")
		if err := ops.Mkdir(dir).RunErr(); err != nil {
			g.T().Fatalf("Mkdir() error: %v", err)
		}
		if err := ops.Put(g.join(dir, "file.txt")).String("x").RunErr(); err != nil {
			g.T().Fatalf("Put(dir/file.txt) error: %v", err)
		}

		err = ops.Remove(dir).RunErr()
		if err != nil {
			// expected
		} else {
			g.T().Error("Remove(non-empty dir) = nil, want error")
		}

		if err := ops.Remove(dir).Recursive().RunErr(); err != nil {
			g.T().Fatalf("Remove(dir).Recursive() error: %v", err)
		}

		exists, err = ops.Exists(dir).RunErr()
		if err != nil {
			g.T().Fatalf("Exists(dir) error: %v", err)
		}
		if exists {
			g.T().Error("directory still exists after Remove().Recursive()")
		}
	})
}

func TestSudoPut(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		if !g.sudo {
			g.T().Skip("sudo unavailable for this target")
		}

		path := "/root/gabricremote2e-" + uuid.NewV4().String() + ".txt"
		ctx := gabric.NewRemoteSudoContext(g.Context(), &gabric.Sudo{})
		ops := gabric.TargetOps(ctx)

		if err := ops.Put(path).String("root data").RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		if got, want := ops.Get(path).String(), "root data"; got != want {
			g.T().Errorf("Get() = %q, want %q", got, want)
		}

		stat, err := ops.Stat(path).RunErr()
		if err != nil {
			g.T().Fatalf("Stat() error: %v", err)
		}
		if stat.User != "root" || stat.Group != "root" {
			g.T().Errorf("owner = %q:%q, want root:root", stat.User, stat.Group)
		}
	})
}

func TestTargetRsyncOps(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		rsync := rsyncOps(g.T(), g.Context())
		requireRsync(g.T())

		local := filepath.Join(g.T().TempDir(), "upload.txt")
		if err := os.WriteFile(local, []byte("target rsync"), 0o600); err != nil {
			g.T().Fatalf("WriteFile() error: %v", err)
		}

		remote := g.targetPath("target-rsync.txt")

		if err := rsync.Local(local).Remote(remote).Upload().RunErr(); err != nil {
			g.T().Fatalf("rsync upload: %v", err)
		}

		if got, want := g.ops().Get(remote).String(), "target rsync"; got != want {
			g.T().Errorf("remote content = %q, want %q", got, want)
		}
	})
}

func TestRsyncUpload(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		rsync := rsyncOps(g.T(), g.Context())
		requireRsync(g.T())

		local := filepath.Join(g.T().TempDir(), "upload.txt")
		if err := os.WriteFile(local, []byte("upload data"), 0o600); err != nil {
			g.T().Fatalf("WriteFile() error: %v", err)
		}

		remote := g.targetPath("rsync-upload.txt")

		if err := rsync.Local(local).Remote(remote).Upload().RunErr(); err != nil {
			g.T().Fatalf("Rsync() error: %v", err)
		}

		if got, want := g.ops().Get(remote).String(), "upload data"; got != want {
			g.T().Errorf("remote content = %q, want %q", got, want)
		}
	})
}

func TestRsyncDownload(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		rsync := rsyncOps(g.T(), g.Context())
		requireRsync(g.T())

		remote := g.targetPath("rsync-download.txt")

		if err := g.ops().Put(remote).String("download data").RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		local := filepath.Join(g.T().TempDir(), "download.txt")

		if err := rsync.Local(local).Remote(remote).Download().RunErr(); err != nil {
			g.T().Fatalf("Rsync() error: %v", err)
		}

		b, err := os.ReadFile(local)
		if err != nil {
			g.T().Fatalf("ReadFile() error: %v", err)
		}
		if got, want := string(b), "download data"; got != want {
			g.T().Errorf("local content = %q, want %q", got, want)
		}
	})
}

func TestRsyncDelete(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		rsync := rsyncOps(g.T(), g.Context())
		requireRsync(g.T())

		localDir := g.T().TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "a.txt"), []byte("a"), 0o600); err != nil {
			g.T().Fatalf("WriteFile(a.txt) error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(localDir, "b.txt"), []byte("b"), 0o600); err != nil {
			g.T().Fatalf("WriteFile(b.txt) error: %v", err)
		}

		remoteDir := g.targetPath("rsync-dir")

		if err := rsync.Local(localDir + "/").Remote(remoteDir + "/").Upload().Delete().RunErr(); err != nil {
			g.T().Fatalf("Rsync() error: %v", err)
		}

		for _, name := range []string{"a.txt", "b.txt"} {
			exists, err := g.ops().Exists(g.join(remoteDir, name)).RunErr()
			if err != nil {
				g.T().Fatalf("Exists(%s) error: %v", name, err)
			}
			if !exists {
				g.T().Errorf("%s does not exist", name)
			}
		}

		if err := g.ops().Put(g.join(remoteDir, "protected.txt")).String("secret").RunErr(); err != nil {
			g.T().Fatalf("Put(protected.txt) error: %v", err)
		}
		if err := os.Remove(filepath.Join(localDir, "a.txt")); err != nil {
			g.T().Fatalf("Remove(a.txt) error: %v", err)
		}

		if err := rsync.Local(localDir + "/").Remote(remoteDir + "/").Upload().Delete().Exclude("protected.txt").RunErr(); err != nil {
			g.T().Fatalf("Rsync() with exclude error: %v", err)
		}

		exists, err := g.ops().Exists(g.join(remoteDir, "a.txt")).RunErr()
		if err != nil {
			g.T().Fatalf("Exists(a.txt) error: %v", err)
		}
		if exists {
			g.T().Error("a.txt still exists after --delete")
		}

		exists, err = g.ops().Exists(g.join(remoteDir, "protected.txt")).RunErr()
		if err != nil {
			g.T().Fatalf("Exists(protected.txt) error: %v", err)
		}
		if !exists {
			g.T().Error("protected.txt was deleted")
		}
	})
}

func TestRsyncRemoteFailure(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		rsync := rsyncOps(g.T(), g.Context())
		requireRsync(g.T())

		local := filepath.Join(g.T().TempDir(), "missing.txt")

		err := rsync.Local(local).Remote(g.targetPath("missing.txt")).Download().RunErr()
		if err != nil {
			// expected
		} else {
			g.T().Fatal("Rsync() of a missing remote file = nil, want error")
		}
	})
}

func TestWorkingDir(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		ops := g.ops()

		ops.WithWorkingDir(g.workDir, func(ctx context.Context) {
			ops := gabric.TargetOps(ctx)
			name := "gabricremote2e-" + uuid.NewV4().String() + ".txt"

			if err := ops.Put(name).String("relative").RunErr(); err != nil {
				g.T().Fatalf("Put() error: %v", err)
			}

			if got, want := ops.Get(name).String(), "relative"; got != want {
				g.T().Errorf("Get() = %q, want %q", got, want)
			}

			exists, err := ops.Exists(name).RunErr()
			if err != nil {
				g.T().Fatalf("Exists() error: %v", err)
			}
			if !exists {
				g.T().Error("Exists() = false, want true")
			}
		})
	})
}

func TestStop(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		if g.stop == nil {
			g.T().Skip("target cannot be stopped")
		}

		g.Stop()

		err := g.ops().Exec("true").RunErr()
		if err != nil {
			// expected
		} else {
			g.T().Fatal("exec succeeded after the target stopped")
		}
	})
}

func TestSymlinks(t *testing.T) {
	forEachTarget(t, func(g *e2eTarget) {
		if !g.symlinks {
			g.T().Skip("target does not support symlinks")
		}

		ops := g.ops()

		file := g.targetPath("symlink-file.txt")
		if err := ops.Put(file).String("x").Mode(0o640).RunErr(); err != nil {
			g.T().Fatalf("Put() error: %v", err)
		}

		link := g.targetPath("symlink-file")
		if err := ops.Exec("ln", "-s", file, link).RunErr(); err != nil {
			g.T().Fatalf("ln -s file: %v", err)
		}

		if got := ops.Exists(link).Link().Run(); !got {
			g.T().Error("Exists(link).Link() = false, want true")
		}
		if got := ops.Exists(file).Link().Run(); got {
			g.T().Error("Exists(file).Link() = true, want false")
		}
		if got := ops.Exists(link).File().Run(); !got {
			g.T().Error("Exists(link).File() = false, want true")
		}
		if got := ops.Exists(link).Dir().Run(); got {
			g.T().Error("Exists(link).Dir() = true, want false")
		}
		if got := ops.Exists(link).Run(); !got {
			g.T().Error("Exists(link) = false, want true")
		}

		stat, err := ops.Stat(link).RunErr()
		if err != nil {
			g.T().Fatalf("Stat(link) error: %v", err)
		}
		if !stat.Mode.IsRegular() {
			g.T().Errorf("link mode = %v, want a regular file", stat.Mode)
		}
		if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
			g.T().Errorf("link mode = %v, want %v", got, want)
		}

		dir := g.targetPath("symlink-dir")
		if err := ops.Mkdir(dir).RunErr(); err != nil {
			g.T().Fatalf("Mkdir() error: %v", err)
		}

		dirLink := g.targetPath("symlink-dir-link")
		if err := ops.Exec("ln", "-s", dir, dirLink).RunErr(); err != nil {
			g.T().Fatalf("ln -s dir: %v", err)
		}

		if got := ops.Exists(dirLink).Dir().Run(); !got {
			g.T().Error("Exists(dirLink).Dir() = false, want true")
		}
		if got := ops.Exists(dirLink).File().Run(); got {
			g.T().Error("Exists(dirLink).File() = true, want false")
		}
		if got := ops.Exists(dirLink).Run(); !got {
			g.T().Error("Exists(dirLink) = false, want true")
		}

		dirStat, err := ops.Stat(dirLink).RunErr()
		if err != nil {
			g.T().Fatalf("Stat(dirLink) error: %v", err)
		}
		if !dirStat.Mode.IsDir() {
			g.T().Error("dirLink Mode.IsDir() = false, want true")
		}

		dangling := g.targetPath("symlink-dangling")
		if err := ops.Exec("ln", "-s", g.targetPath("symlink-missing"), dangling).RunErr(); err != nil {
			g.T().Fatalf("ln -s dangling: %v", err)
		}

		if got := ops.Exists(dangling).Link().Run(); !got {
			g.T().Error("Exists(dangling).Link() = false, want true")
		}
		if got := ops.Exists(dangling).Run(); got {
			g.T().Error("Exists(dangling) = true, want false")
		}
		if got := ops.Exists(dangling).File().Run(); got {
			g.T().Error("Exists(dangling).File() = true, want false")
		}
		_, err = ops.Stat(dangling).RunErr()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				g.T().Errorf("Stat(dangling) error = %v, want fs.ErrNotExist", err)
			}
		} else {
			g.T().Error("Stat(dangling) error = nil, want fs.ErrNotExist")
		}

		listDir := g.targetPath("symlink-list")
		if err := ops.Mkdir(listDir).RunErr(); err != nil {
			g.T().Fatalf("Mkdir(list) error: %v", err)
		}
		if err := ops.Exec("ln", "-s", dir, g.join(listDir, "dirlink")).RunErr(); err != nil {
			g.T().Fatalf("ln -s list: %v", err)
		}

		entries := ops.List(listDir)
		if len(entries) != 1 {
			g.T().Fatalf("List() = %+v, want one entry", entries)
		}
		if entries[0].Name != "dirlink" {
			g.T().Errorf("List()[0].Name = %q, want %q", entries[0].Name, "dirlink")
		}
		if entries[0].IsDir {
			g.T().Error("List()[0].IsDir = true, want false")
		}

		if err := ops.Remove(link).RunErr(); err != nil {
			g.T().Fatalf("Remove(link) error: %v", err)
		}
		if err := ops.Remove(dirLink).RunErr(); err != nil {
			g.T().Fatalf("Remove(dirLink) error: %v", err)
		}

		if got := ops.Exists(link).Run(); got {
			g.T().Error("Exists(link) = true after Remove, want false")
		}
		if got := ops.Exists(dirLink).Run(); got {
			g.T().Error("Exists(dirLink) = true after Remove, want false")
		}
		if got := ops.Exists(file).File().Run(); !got {
			g.T().Error("Exists(file) = false after Remove(link), want true")
		}
		if got := ops.Exists(dir).Dir().Run(); !got {
			g.T().Error("Exists(dir) = false after Remove(dirLink), want true")
		}
	})
}
