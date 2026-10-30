package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/rsync"
	"github.com/kballard/go-shellquote"
)

var (
	homelabDirFlag = gabric.Default.FlagSet.String("homelab-dir", ".", "homelab source directory")
	remoteDirFlag  = gabric.Default.FlagSet.String("remote-dir", "", "remote deployment directory (default $HOME/homelab)")
)

type deployOptions struct {
	localDir  string
	remoteDir string
	gabArgs   []string
	capture   bool
}

// DeployLocal builds the homelab binary for the target, uploads the directory,
// and runs ./bin/homelab with the exact arguments supplied as the task argument.
// It uses -ssh-host when set and otherwise configures the Vagrant VM.
var DeployLocal = gabric.Task("deploy-local", func(ctx context.Context) {
	if dryrun.Enabled(ctx) {
		gabric.Logger(ctx).Info("dry run: would build and deploy homelab")
		return
	}

	gabArgs, err := shellquote.Split(gabric.GetCurrentTaskArg(ctx))
	gabric.Must(err)

	gabric.Must(withVagrantIfNoHost(ctx, func(ctx context.Context) error {
		_, err := deployLocal(ctx, deployOptions{
			localDir:  *homelabDirFlag,
			remoteDir: *remoteDirFlag,
			gabArgs:   gabArgs,
		})
		return err
	}))
})

func deployLocal(ctx context.Context, opts deployOptions) (string, error) {
	ctx = gabric.NewRemoteWorkingDirContext(ctx, "")
	ctx = gabric.NewRemoteSudoContext(ctx, nil)

	localDir, err := filepath.Abs(opts.localDir)
	if err != nil {
		return "", err
	}

	arch, err := remoteArch(ctx)
	if err != nil {
		return "", err
	}

	goarch, err := goArchFor(arch)
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(localDir, "bin")
	if err := gabric.Local(ctx).Mkdir(binDir).Recursive().RunErr(); err != nil {
		return "", err
	}

	err = gabric.Local(ctx).WithWorkingDirErr(localDir, func(ctx context.Context) error {
		return gabric.Local(ctx).
			Exec("go", "build", "-trimpath", "-o", "bin/homelab", ".").
			WithEnv(
				gabric.Env("GOOS", "linux"),
				gabric.Env("GOARCH", goarch),
				gabric.Env("CGO_ENABLED", "0"),
			).
			RunErr()
	})
	if err != nil {
		return "", err
	}

	remoteDir := opts.remoteDir
	if remoteDir == "" {
		remoteDir, err = defaultRemoteDir(ctx)
		if err != nil {
			return "", err
		}
	}

	ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)

	if _, err := reconcile.RunNode(ctx, deployNode(localDir, remoteDir)); err != nil {
		return "", err
	}

	args := append([]string{"./bin/homelab"}, opts.gabArgs...)

	var out string

	err = gabric.Remote(ctx).WithWorkingDirErr(remoteDir, func(ctx context.Context) error {
		e := gabric.Remote(ctx).Exec(args...)
		if opts.capture {
			e = e.Capture()
		}

		runErr := e.RunErr()
		if opts.capture {
			out = e.StdoutStr() + e.StderrStr()
		}

		return runErr
	})

	return out, err
}

func remoteArch(ctx context.Context) (string, error) {
	e := gabric.Remote(ctx).Exec("uname", "-m").CaptureStdout()
	if err := e.RunErr(); err != nil {
		return "", err
	}

	arch := strings.TrimSpace(e.StdoutStr())
	if arch == "" {
		return "", errors.New("deploy: empty remote architecture")
	}

	return arch, nil
}

func defaultRemoteDir(ctx context.Context) (string, error) {
	e := gabric.Remote(ctx).Exec("sh", "-c", `printf %s "$HOME"`).CaptureStdout()
	if err := e.RunErr(); err != nil {
		return "", err
	}

	home := strings.TrimSpace(e.StdoutStr())
	if home == "" {
		return "", errors.New("deploy: could not resolve remote home")
	}

	return path.Join(home, "homelab"), nil
}

// deployRsyncPkg installs rsync as root with non-interactive sudo before the
// bootstrap has run.
var deployRsyncPkg = rsyncPkg.With(gabric.WithRemoteSudo(&gabric.Sudo{
	Flags: []string{"-n"},
}))

// deployNode ensures rsync, creates the remote directory, and uploads the
// directory. Only the rsync package node runs as root; Dir and Rsync keep the
// SSH user so uploaded 0600 files remain readable by the local-mode binary.
func deployNode(localDir, remoteDir string) *reconcile.Node[reconcile.Void] {
	return reconcile.Group("deploy").
		Contains(deployRsyncPkg).
		Contains(modules.Dir(S(remoteDir)).Build().Node()).
		Contains(rsync.Sync(S(localDir + "/")).
			Remote(S(remoteDir)).
			Exclude(input.List(".git", ".vagrant", "secrets.plain", "secrets/keys.txt", ".DS_Store")).
			Build().Node()).
		Node()
}

func goArchFor(uname string) (string, error) {
	switch uname {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	case "armv7l":
		return "arm", nil
	default:
		return "", fmt.Errorf("unsupported remote architecture %q", uname)
	}
}
