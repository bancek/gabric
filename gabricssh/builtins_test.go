package gabricssh

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/sshremote"
)

func TestWithSSH(t *testing.T) {
	var got context.Context

	if err := WithSSH(testContext(t), func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("WithSSH() error: %v", err)
	}

	if !HasConfig(got) {
		t.Error("SSH config was not installed")
	}
	if !HasManager(got) {
		t.Error("SSH manager was not installed")
	}
	if !gabric.HasRemoteRunner(got) {
		t.Error("SSH runner was not installed")
	}
	if !gabric.HasRsyncRsh(got) {
		t.Error("SSH rsync resolver was not installed")
	}
}

func TestWithSSHKeepsExisting(t *testing.T) {
	config := sshremote.NewConfig()
	manager := sshremote.NewRemoteManager()
	defer manager.Close()

	var runnerCalled bool

	runner := func(context.Context, string, io.Reader, io.Writer, io.Writer) error {
		runnerCalled = true
		return nil
	}
	wantRsh := gabric.RsyncRsh{HostSpec: "custom@example.com", Rsh: "custom-rsh"}

	ctx := testContext(t)
	ctx = NewConfigContext(ctx, config)
	ctx = NewManagerContext(ctx, manager)
	ctx = gabric.NewRemoteRunnerContext(ctx, runner)
	ctx = gabric.NewRsyncRshContext(ctx, func(context.Context) (gabric.RsyncRsh, error) {
		return wantRsh, nil
	})

	var got context.Context

	if err := WithSSH(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("WithSSH() error: %v", err)
	}

	if GetConfig(got) != config {
		t.Error("existing config was replaced")
	}
	if GetManager(got) != manager {
		t.Error("existing manager was replaced")
	}

	if err := gabric.GetRemoteRunner(got)(got, "", nil, nil, nil); err != nil {
		t.Fatalf("custom runner error: %v", err)
	}
	if !runnerCalled {
		t.Error("existing runner was replaced")
	}

	rsh, err := gabric.GetRsyncRsh(got)
	if err != nil {
		t.Fatalf("GetRsyncRsh() error: %v", err)
	}
	if rsh != wantRsh {
		t.Errorf("GetRsyncRsh() = %#v, want %#v", rsh, wantRsh)
	}
}

func TestWithSSHIdentity(t *testing.T) {
	config := sshremote.NewConfig().WithPrivateKeysPaths([]string{"default"})
	ctx := NewConfigContext(testContext(t), config)

	flags := &sshInstallFlags{identities: []string{"key1", "key2"}}

	var got context.Context

	if err := withSSHIdentity(flags)(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("withSSHIdentity() error: %v", err)
	}

	want := []string{"default", "key1", "key2"}
	if keys := GetConfig(got).PrivateKeysPaths(); !slices.Equal(keys, want) {
		t.Errorf("PrivateKeysPaths = %v, want %v", keys, want)
	}
}

func TestWithSSHHost(t *testing.T) {
	ctx := NewConfigContext(testContext(t), sshremote.NewConfig())
	flags := &sshInstallFlags{host: "deploy@example.com:2222"}

	var got context.Context

	if err := withSSHHost(flags)(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("withSSHHost() error: %v", err)
	}

	if !HasHost(got) {
		t.Fatal("host was not installed")
	}

	resolved := GetHost(got)
	if got, want := resolved.Hostname(), "example.com"; got != want {
		t.Errorf("Hostname = %q, want %q", got, want)
	}
	if got, want := resolved.Port(), 2222; got != want {
		t.Errorf("Port = %d, want %d", got, want)
	}
}
