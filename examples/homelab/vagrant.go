package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/sshremote"
)

// vagrantUp boots the VM (destroying it first when fresh) and resolves its SSH
// target.
func vagrantUp(ctx context.Context, fresh bool) (sshremote.HostConfig, error) {
	if _, err := exec.LookPath("vagrant"); err != nil {
		return sshremote.HostConfig{}, fmt.Errorf("vagrant not found: %w", err)
	}

	if fresh {
		if err := vagrantDestroy(ctx); err != nil {
			gabric.Logger(ctx).Warn("vagrant destroy failed", "error", err)
		}
	}

	if err := runVagrant(ctx, "up"); err != nil {
		return sshremote.HostConfig{}, err
	}

	return vagrantSSHConfig(ctx)
}

// vagrantSSHConfig resolves the Host default block from vagrant ssh-config.
func vagrantSSHConfig(ctx context.Context) (sshremote.HostConfig, error) {
	sshConfigBytes, err := captureVagrant(ctx, "ssh-config")
	if err != nil {
		return sshremote.HostConfig{}, err
	}

	return vagrantHostConfig(sshConfigBytes)
}

// vagrantHostConfig resolves the Host default block from the Vagrant
// ssh_config content with sshremote.
func vagrantHostConfig(sshConfigBytes []byte) (sshremote.HostConfig, error) {
	config, err := sshremote.NewDefaultConfig()
	if err != nil {
		return sshremote.HostConfig{}, err
	}

	config = config.
		WithSSHConfigContent(sshConfigBytes).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

	host, err := config.ResolveHost("default")
	if err != nil {
		return sshremote.HostConfig{}, err
	}

	if host.Hostname() == "" || host.Hostname() == "default" {
		return sshremote.HostConfig{}, fmt.Errorf("vagrant ssh-config: missing HostName")
	}

	return sshremote.NewHostConfig(host, config), nil
}

// vagrantDestroy destroys the VM.
func vagrantDestroy(ctx context.Context) error {
	return runVagrant(ctx, "destroy", "-f")
}

func runVagrant(ctx context.Context, args ...string) error {
	dir, err := homelabDir()
	if err != nil {
		return err
	}

	return gabric.Local(ctx).WithWorkingDirErr(dir, func(ctx context.Context) error {
		return gabric.Local(ctx).Exec(append([]string{"vagrant"}, args...)...).RunErr()
	})
}

func captureVagrant(ctx context.Context, args ...string) (out []byte, err error) {
	dir, err := homelabDir()
	if err != nil {
		return nil, err
	}

	err = gabric.Local(ctx).WithWorkingDirErr(dir, func(ctx context.Context) error {
		e := gabric.Local(ctx).Exec(append([]string{"vagrant"}, args...)...).CaptureStdout()
		runErr := e.RunErr()
		out = e.StdoutBytes()

		return runErr
	})
	return out, err
}

func homelabDir() (string, error) {
	return filepath.Abs(*homelabDirFlag)
}

// withVagrantHostConfig configures SSH for an already-resolved host and runs
// next.
func withVagrantHostConfig(ctx context.Context, hostConfig sshremote.HostConfig, next middleware.Next) error {
	ctx = gabricssh.NewConfigContext(ctx, hostConfig.Config())
	ctx = gabricssh.NewHostContext(ctx, hostConfig.Host())
	ctx = gabricssh.NewRunnerContext(ctx)
	return gabricssh.WithSSH(ctx, next)
}

// withVagrantRemote boots the VM and configures SSH for it. A dry run skips the
// boot and leaves the decision to the task body.
func withVagrantRemote(ctx context.Context, next middleware.Next) error {
	if dryrun.Enabled(ctx) {
		return next(ctx)
	}

	hostConfig, err := vagrantUp(ctx, false)
	if err != nil {
		return err
	}

	return withVagrantHostConfig(ctx, hostConfig, next)
}

// withVagrantIfNoHost configures the Vagrant VM only when no SSH host is set.
func withVagrantIfNoHost(ctx context.Context, next middleware.Next) error {
	if gabricssh.HasHost(ctx) {
		return next(ctx)
	}

	return withVagrantRemote(ctx, next)
}

func requireRealRun(ctx context.Context, name string) {
	if dryrun.Enabled(ctx) {
		panic(name + ": do not run with -dry-run")
	}
}

// VMUp boots the Vagrant VM.
var VMUp = gabric.Task("vm-up", func(ctx context.Context) {
	requireRealRun(ctx, "vm-up")

	hostConfig, err := vagrantUp(ctx, false)
	gabric.Must(err)

	gabric.Logger(ctx).Info("VM ready", "ssh", hostConfig.String())
})

// VMShell opens an interactive shell on the Vagrant VM, booting it if needed.
var VMShell = gabric.With(withVagrantRemote).Task("vm-shell", func(ctx context.Context) {
	requireRealRun(ctx, "vm-shell")

	gabricssh.Shell(ctx).Run()
})

// VMDown destroys the Vagrant VM.
var VMDown = gabric.Task("vm-down", func(ctx context.Context) {
	requireRealRun(ctx, "vm-down")

	gabric.Must(vagrantDestroy(ctx))
})
