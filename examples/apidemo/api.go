package apidemo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bancek/gabric"
)

// Register registers the api task on g.
func Register(g *gabric.Gabric) func(context.Context) {
	return g.Task("api", Run)
}

// Run exercises the target operations for the current transport.
func Run(ctx context.Context) {
	if _, err := exec.LookPath("rsync"); err != nil {
		panic("apidemo requires rsync on the host: " + err.Error())
	}

	ops := gabric.TargetOps(ctx)

	remoteDir := strings.TrimSpace(ops.Exec("mktemp", "-d").CaptureStdout().Run().StdoutStr())
	if remoteDir == "" {
		panic("apidemo: mktemp returned an empty path")
	}
	defer ops.Remove(remoteDir).Recursive().Force().Run()

	sub := path.Join(remoteDir, "sub")
	ops.Mkdir(sub).Recursive().Mode(0o750).Run()

	dirStat := ops.Stat(sub).Run()
	if !dirStat.Mode.IsDir() {
		panic("apidemo: Mkdir did not create a directory")
	}

	file := path.Join(sub, "file.txt")
	ops.Put(file).String("hello remote api").Mode(0o600).Run()

	if got := ops.Get(file).String(); got != "hello remote api" {
		panic(fmt.Sprintf("apidemo: Get() = %q, want %q", got, "hello remote api"))
	}

	if !ops.Exists(file).File().Run() {
		panic("apidemo: Exists(file).File() = false")
	}
	if !ops.Exists(sub).Dir().Run() {
		panic("apidemo: Exists(dir).Dir() = false")
	}
	if ops.Exists(path.Join(remoteDir, "missing")).Run() {
		panic("apidemo: Exists(missing) = true")
	}

	fileStat := ops.Stat(file).Run()
	if got, want := fileStat.Mode.Perm(), os.FileMode(0o600); got != want {
		panic(fmt.Sprintf("apidemo: Stat().Mode.Perm() = %v, want %v", got, want))
	}

	entries := ops.List(sub)
	if !slices.ContainsFunc(entries, func(entry gabric.ListEntry) bool {
		return entry.Name == "file.txt" && !entry.IsDir
	}) {
		panic(fmt.Sprintf("apidemo: List() = %#v, want file.txt", entries))
	}

	if got := strings.TrimSpace(ops.Exec("sh", "-c", "printf exec-ok").CaptureStdout().Run().StdoutStr()); got != "exec-ok" {
		panic(fmt.Sprintf("apidemo: Exec() = %q, want %q", got, "exec-ok"))
	}

	ops.WithWorkingDir(sub, func(ctx context.Context) {
		if got := strings.TrimSpace(gabric.TargetOps(ctx).Exec("pwd").CaptureStdout().Run().StdoutStr()); got != sub {
			panic(fmt.Sprintf("apidemo: working dir = %q, want %q", got, sub))
		}
	})

	localDir, err := os.MkdirTemp("", "gabric-apidemo-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(localDir)

	localUpload := filepath.Join(localDir, "upload.txt")
	if err := os.WriteFile(localUpload, []byte("rsync upload"), 0o600); err != nil {
		panic(err)
	}

	remoteRsyncDir := path.Join(remoteDir, "rsync")
	ops.Mkdir(remoteRsyncDir).Recursive().Run()

	rsyncOps, err := gabric.TargetRsyncOps(ctx)
	if err != nil {
		panic(err)
	}

	remoteRsyncFile := path.Join(remoteRsyncDir, "upload.txt")
	rsyncOps.Local(localUpload).Remote(remoteRsyncFile).Upload().Run()

	if got := ops.Get(remoteRsyncFile).String(); got != "rsync upload" {
		panic(fmt.Sprintf("apidemo: rsync upload content = %q, want %q", got, "rsync upload"))
	}

	localDownload := filepath.Join(localDir, "download.txt")
	rsyncOps.Local(localDownload).Remote(remoteRsyncFile).Download().Run()

	content, err := os.ReadFile(localDownload)
	if err != nil {
		panic(err)
	}
	if got, want := string(content), "rsync upload"; got != want {
		panic(fmt.Sprintf("apidemo: rsync download content = %q, want %q", got, want))
	}

	gabric.Logger(ctx).Info("apidemo passed")
}
