package gabricssh

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bancek/gabric/sshremote"
)

func TestGetHost(t *testing.T) {
	host := sshremote.NewHost("example.com")

	ctx := NewHostContext(testContext(t), host)

	if GetHost(ctx) != host {
		t.Error("GetHost() did not return the host from context")
	}

	if !HasHost(ctx) {
		t.Error("HasHost() = false, want true")
	}

	if HasHost(context.Background()) {
		t.Error("HasHost() = true, want false")
	}
}

func TestWithHost(t *testing.T) {
	ctx := NewConfigContext(testContext(t), sshremote.NewConfig())

	var got context.Context

	if err := WithHost("example.com")(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("WithHost() error: %v", err)
	}

	if !HasHost(got) {
		t.Fatal("WithHost() did not set a host")
	}
	if got, want := GetHost(got).Hostname(), "example.com"; got != want {
		t.Errorf("Hostname = %q, want %q", got, want)
	}
}

func TestGetHostPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("GetHost() did not panic")
		}
	}()

	GetHost(context.Background())
}

func TestGetHostErrNoHost(t *testing.T) {
	if _, err := GetHostErr(context.Background()); err == nil {
		t.Error("GetHostErr() with no host = nil error, want error")
	}
}

const hostNameSSHConfig = `
Host example
    HostName example.com
    User deploy
    Port 2222
`

func hostNameContext(t *testing.T, spec string, policy sshremote.StrictHostKeyChecking) context.Context {
	t.Helper()

	sshConfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfigPath, []byte(hostNameSSHConfig), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	config := sshremote.NewConfig().
		WithSSHConfigPath(sshConfigPath).
		WithStrictHostKeyChecking(policy)

	ctx := NewConfigContext(testContext(t), config)

	return NewHostNameContext(ctx, spec)
}

func TestGetHostFromName(t *testing.T) {
	ctx := hostNameContext(t, "example", sshremote.StrictHostKeyCheckingYes)

	host := GetHost(ctx)
	if host.Hostname() != "example.com" {
		t.Errorf("Hostname = %q, want %q", host.Hostname(), "example.com")
	}
	if host.User() != "deploy" {
		t.Errorf("User = %q, want %q", host.User(), "deploy")
	}
	if host.Port() != 2222 {
		t.Errorf("Port = %d, want %d", host.Port(), 2222)
	}
	if !HasHost(ctx) {
		t.Error("HasHost() = false, want true")
	}

	// Resolution is memoized: the same host is returned on every call.
	if GetHost(ctx) != host {
		t.Error("GetHost() returned a different host on the second call")
	}
}

func TestGetHostFromNamePortOverride(t *testing.T) {
	ctx := hostNameContext(t, "example:2022", sshremote.StrictHostKeyCheckingYes)

	if host := GetHost(ctx); host.Port() != 2022 {
		t.Errorf("Port = %d, want %d (the host spec port)", host.Port(), 2022)
	}
}

func TestGetHostUsesCurrentConfig(t *testing.T) {
	// The config is set before the first GetHost, so HostConfig resolves to it.
	ctx := hostNameContext(t, "example", sshremote.StrictHostKeyCheckingNo)

	if hc := GetHostConfig(ctx); hc.StrictHostKeyChecking() != sshremote.StrictHostKeyCheckingNo {
		t.Errorf("StrictHostKeyChecking = %q, want %q (from the config)", hc.StrictHostKeyChecking(), sshremote.StrictHostKeyCheckingNo)
	}
}

func TestHostContextLogValue(t *testing.T) {
	hc := &hostContext{spec: "example.com:2222"}

	if got := hc.LogValue().String(); got != "example.com:2222" {
		t.Errorf("LogValue() = %q, want %q", got, "example.com:2222")
	}

	host := sshremote.NewHost("example.com").WithUser("deploy").WithPort(2222)
	hc.resolved.Store(host)

	if got := hc.LogValue().String(); got != host.String() {
		t.Errorf("LogValue() = %q, want %q", got, host.String())
	}
}
