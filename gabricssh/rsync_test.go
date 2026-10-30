package gabricssh

import (
	"context"
	"strings"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/sshremote"
)

func TestRsyncRsh(t *testing.T) {
	host := sshremote.NewHost("example.com").
		WithUser("deploy").
		WithPort(2222).
		WithKnownHostsPaths([]string{"/known_hosts"})
	config := sshremote.NewConfig()

	ctx := NewConfigContext(context.Background(), config)
	ctx = NewHostContext(ctx, host)

	rsh, err := sshRsyncRsh(ctx)
	if err != nil {
		t.Fatalf("sshRsyncRsh() error: %v", err)
	}

	if got, want := rsh.HostSpec, "deploy@example.com"; got != want {
		t.Errorf("HostSpec = %q, want %q", got, want)
	}
	if !strings.HasPrefix(rsh.Rsh, "ssh ") {
		t.Errorf("Rsh = %q, want an ssh command", rsh.Rsh)
	}
	if !strings.Contains(rsh.Rsh, "-p 2222") {
		t.Errorf("Rsh = %q, want the host port", rsh.Rsh)
	}
}

func TestNewRunnerContext(t *testing.T) {
	host := sshremote.NewHost("example.com")
	config := sshremote.NewConfig()

	ctx := NewConfigContext(context.Background(), config)
	ctx = NewHostContext(ctx, host)
	ctx = NewRunnerContext(ctx)

	if !gabric.HasRemoteRunner(ctx) {
		t.Error("HasRemoteRunner() = false, want true")
	}
	if !gabric.HasRsyncRsh(ctx) {
		t.Error("HasRsyncRsh() = false, want true")
	}
}
