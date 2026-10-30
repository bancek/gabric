package linux

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestUserResourceName(t *testing.T) {
	if got, want := User(input.Static("testuser")).Build().Node().Name(), "linux.user:testuser"; got != want {
		t.Errorf("default name = %q, want %q", got, want)
	}

	got := User(input.WithName(input.Static("testuser"), "deploy")).Build().Node().Name()
	if want := "linux.user:deploy"; got != want {
		t.Errorf("WithName() = %q, want %q", got, want)
	}
}

func TestUserNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, User(input.Static("testuser")).Build().Node())

	gt.Remote().AssertCommands(sudoCommand(gt, "getent", "passwd", "testuser"))
	gt.Remote().AssertCalls()
}

func TestUserDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})
	gt.Remote().Handle("useradd", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, User(input.Static("testuser")).Build().Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "useradd", "--user-group", "--create-home", "--shell", "/bin/bash", "testuser"),
	)
	gt.Remote().AssertCalls()
}

func TestUserShellDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/sh"}
	})
	gt.Remote().Handle("usermod", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, User(input.Static("testuser")).Shell(input.Static("/bin/zsh")).Build().Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "usermod", "--shell", "/bin/zsh", "testuser"),
	)
	gt.Remote().AssertCalls()
}

func TestUserGroupsDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser"}
	})
	gt.Remote().Handle("usermod", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, User(input.Static("testuser")).Groups(input.List("sudo")).Build().Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "id", "-nG", "testuser"),
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "usermod", "-aG", "sudo", "testuser"),
	)
	gt.Remote().AssertCalls()
}

func TestUserGroupsNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser sudo"}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, User(input.Static("testuser")).Groups(input.List("sudo")).Build().Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "id", "-nG", "testuser"),
	)
	gt.Remote().AssertCalls()
}
