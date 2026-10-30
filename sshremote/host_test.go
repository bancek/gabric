package sshremote_test

import (
	"os/user"
	"reflect"
	"testing"

	"github.com/bancek/gabric/sshremote"
)

func TestParseHost(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want *sshremote.Host
	}{
		{
			name: "hostname",
			in:   "test",
			want: sshremote.NewHost("test"),
		},
		{
			name: "ip",
			in:   "127.0.0.1",
			want: sshremote.NewHost("127.0.0.1"),
		},
		{
			name: "ip with port",
			in:   "127.0.0.1:2222",
			want: sshremote.NewHost("127.0.0.1").WithPort(2222),
		},
		{
			name: "user and host",
			in:   "test@127.0.0.1",
			want: sshremote.NewHost("127.0.0.1").WithUser("test"),
		},
		{
			name: "user, host and port",
			in:   "user@127.0.0.1:2222",
			want: sshremote.NewHost("127.0.0.1").WithPort(2222).WithUser("user"),
		},
		{
			name: "ipv6 with port",
			in:   "[::1]:2222",
			want: sshremote.NewHost("::1").WithPort(2222),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sshremote.ParseHost(tt.in)
			if err != nil {
				t.Fatalf("ParseHost(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseHost(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseHostError(t *testing.T) {
	if _, err := sshremote.ParseHost("user:pass@127.0.0.1"); err == nil {
		t.Fatal("ParseHost() with password: expected error, got nil")
	}
}

func TestHostPort(t *testing.T) {
	tests := []struct {
		name string
		port int
		want int
	}{
		{name: "default", port: 0, want: 22},
		{name: "custom", port: 2222, want: 2222},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := sshremote.NewHost("example.com").WithPort(tt.port)
			if got := h.Port(); got != tt.want {
				t.Errorf("Port() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHostUser(t *testing.T) {
	h := sshremote.NewHost("example.com").WithUser("deploy")
	if got := h.User(); got != "deploy" {
		t.Errorf("User() = %q, want %q", got, "deploy")
	}

	current, err := user.Current()
	if err != nil {
		t.Skipf("user.Current() error: %v", err)
	}

	h = sshremote.NewHost("example.com")
	if got := h.User(); got != current.Username {
		t.Errorf("User() = %q, want current OS user %q", got, current.Username)
	}
}

func TestHostGetHost(t *testing.T) {
	tests := []struct {
		name string
		host *sshremote.Host
		want string
	}{
		{
			name: "default port",
			host: sshremote.NewHost("example.com"),
			want: "example.com:22",
		},
		{
			name: "custom port",
			host: sshremote.NewHost("example.com").WithPort(2222),
			want: "example.com:2222",
		},
		{
			name: "ipv6",
			host: sshremote.NewHost("::1"),
			want: "[::1]:22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.host.GetHost(); got != tt.want {
				t.Errorf("GetHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostSSHHost(t *testing.T) {
	tests := []struct {
		name string
		host *sshremote.Host
		want string
	}{
		{
			name: "user and host",
			host: sshremote.NewHost("example.com").WithUser("deploy"),
			want: "deploy@example.com",
		},
		{
			name: "no port",
			host: sshremote.NewHost("example.com").WithUser("deploy").WithPort(2222),
			want: "deploy@example.com",
		},
		{
			name: "ipv6 with user",
			host: sshremote.NewHost("::1").WithUser("deploy"),
			want: "deploy@[::1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.host.SSHHost(); got != tt.want {
				t.Errorf("SSHHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostString(t *testing.T) {
	tests := []struct {
		name string
		host *sshremote.Host
		want string
	}{
		{
			name: "user, host and port",
			host: sshremote.NewHost("example.com").WithUser("deploy").WithPort(2222),
			want: "deploy@example.com:2222",
		},
		{
			name: "display name",
			host: sshremote.NewHost("example.com").WithUser("deploy").WithDisplayName("web1"),
			want: "web1 (deploy@example.com:22)",
		},
		{
			name: "ipv6",
			host: sshremote.NewHost("::1").WithUser("deploy"),
			want: "deploy@[::1]:22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.host.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostSigners(t *testing.T) {
	signer := newTestSigner(t)

	host := sshremote.NewHost("example.com").WithSigners(signer)
	if got := host.Signers(); len(got) != 1 {
		t.Fatalf("len(Signers()) = %d, want 1", len(got))
	}

	if got := host.Clone().Signers(); len(got) != 1 {
		t.Fatalf("len(Clone().Signers()) = %d, want 1", len(got))
	}

	if got := host.WithSigners().Signers(); len(got) != 0 {
		t.Errorf("len(WithSigners().Signers()) = %d, want 0", len(got))
	}

	signers := host.Signers()
	signers[0] = nil

	if host.Signers()[0] == nil {
		t.Error("Signers() returned the live slice")
	}
}

func TestHostClone(t *testing.T) {
	proxy := sshremote.NewHost("jump.example.com")

	host := sshremote.NewHost("10.0.0.5").
		WithDisplayName("web1").
		WithPort(2222).
		WithUser("deploy").
		WithProxyCommand("ssh jump nc %h %p").
		WithTransport(sshremote.JumpTransport{Host: proxy}).
		WithPrivateKeysPaths([]string{"/home/me/.ssh/deploy_key"}).
		WithKnownHostsPaths([]string{"/home/me/.ssh/known_hosts", "/home/me/.ssh/known_hosts2"}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes)

	clone := host.Clone()

	if clone == host {
		t.Fatal("Clone() returned the same pointer")
	}

	if clone.Hostname() != host.Hostname() ||
		clone.Port() != host.Port() ||
		clone.User() != host.User() ||
		clone.DisplayName() != host.DisplayName() {
		t.Error("Clone() did not preserve the host fields")
	}

	if jumpHost(t, clone.Transport()) != jumpHost(t, host.Transport()) {
		t.Error("Clone() should keep the jump host pointer")
	}

	changed := clone.
		WithPrivateKeysPaths([]string{"/other"}).
		WithKnownHostsPaths([]string{"/other_known_hosts"}).
		WithHostname("other.example.com")

	if host.PrivateKeysPaths()[0] != "/home/me/.ssh/deploy_key" {
		t.Errorf("WithPrivateKeysPaths changed the original keys: %v", host.PrivateKeysPaths())
	}
	if host.KnownHostsPaths()[0] != "/home/me/.ssh/known_hosts" {
		t.Errorf("WithKnownHostsPaths changed the original known hosts: %v", host.KnownHostsPaths())
	}
	if host.Hostname() != "10.0.0.5" {
		t.Errorf("WithHostname changed the original host: %q", host.Hostname())
	}
	if changed.Hostname() != "other.example.com" {
		t.Errorf("changed hostname = %q, want other.example.com", changed.Hostname())
	}
}
