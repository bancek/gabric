package sshremote_test

import (
	"net"
	"slices"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric/sshremote"
)

func TestHostConfigPrivateKeysPaths(t *testing.T) {
	host := sshremote.NewHost("example.com").
		WithPrivateKeysPaths([]string{"/host/key1", "/host/key2"})
	config := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/config/key"})

	hc := sshremote.NewHostConfig(host, config)

	want := []string{"/host/key1", "/host/key2", "/config/key"}
	if got := hc.PrivateKeysPaths(); !slices.Equal(got, want) {
		t.Errorf("PrivateKeysPaths() = %v, want %v", got, want)
	}
}

func TestHostConfigSigners(t *testing.T) {
	hostSigner := newTestSigner(t)
	configSigner := newTestSigner(t)

	host := sshremote.NewHost("example.com").WithSigners(hostSigner)
	config := sshremote.NewConfig().WithSigners(configSigner)

	signers := sshremote.NewHostConfig(host, config).Signers()
	if len(signers) != 2 {
		t.Fatalf("len(Signers()) = %d, want 2", len(signers))
	}

	if got, want := ssh.FingerprintSHA256(signers[0].PublicKey()), ssh.FingerprintSHA256(hostSigner.PublicKey()); got != want {
		t.Errorf("signer[0] = %s, want host signer %s", got, want)
	}
	if got, want := ssh.FingerprintSHA256(signers[1].PublicKey()), ssh.FingerprintSHA256(configSigner.PublicKey()); got != want {
		t.Errorf("signer[1] = %s, want config signer %s", got, want)
	}
}

func TestHostConfigKnownHostsPaths(t *testing.T) {
	config := sshremote.NewConfig().
		WithKnownHostsPaths([]string{"/config/known_hosts"})

	host := sshremote.NewHost("example.com").
		WithKnownHostsPaths([]string{"/host/known_hosts"})
	if got, want := sshremote.NewHostConfig(host, config).KnownHostsPaths(), []string{"/host/known_hosts"}; !slices.Equal(got, want) {
		t.Errorf("KnownHostsPaths() = %v, want %v", got, want)
	}

	host = sshremote.NewHost("example.com")
	if got, want := sshremote.NewHostConfig(host, config).KnownHostsPaths(), []string{"/config/known_hosts"}; !slices.Equal(got, want) {
		t.Errorf("KnownHostsPaths() = %v, want %v", got, want)
	}
}

func TestHostConfigStrictHostKeyChecking(t *testing.T) {
	tests := []struct {
		name   string
		host   *sshremote.Host
		config *sshremote.Config
		want   sshremote.StrictHostKeyChecking
	}{
		{
			name:   "host override",
			host:   sshremote.NewHost("example.com").WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo),
			config: sshremote.NewConfig().WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes),
			want:   sshremote.StrictHostKeyCheckingNo,
		},
		{
			name:   "config fallback",
			host:   sshremote.NewHost("example.com"),
			config: sshremote.NewConfig().WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew),
			want:   sshremote.StrictHostKeyCheckingAcceptNew,
		},
		{
			name:   "both empty",
			host:   sshremote.NewHost("example.com"),
			config: sshremote.NewConfig(),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sshremote.NewHostConfig(tt.host, tt.config).StrictHostKeyChecking(); got != tt.want {
				t.Errorf("StrictHostKeyChecking() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostConfigHostKeyCallback(t *testing.T) {
	called := false

	config := sshremote.NewConfig().WithHostKeyCallback(func(hc sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		called = true

		if hc.Host().Hostname() != "example.com" {
			t.Errorf("callback host = %q, want example.com", hc.Host().Hostname())
		}

		return func(string, net.Addr, ssh.PublicKey) error { return nil }, nil
	})

	host := sshremote.NewHost("example.com")
	if _, err := sshremote.NewHostConfig(host, config).HostKeyCallback(); err != nil {
		t.Fatalf("HostKeyCallback() error: %v", err)
	}
	if !called {
		t.Error("HostKeyCallback() did not call the configured callback")
	}
}

func TestHostConfigSSHArgs(t *testing.T) {
	tests := []struct {
		name   string
		host   *sshremote.Host
		config *sshremote.Config
		want   []string
	}{
		{
			name:   "empty",
			host:   sshremote.NewHost("example.com").WithUser("deploy"),
			config: sshremote.NewConfig(),
			want:   []string{},
		},
		{
			name: "host and config keys, port and known hosts",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithPort(2222).
				WithPrivateKeysPaths([]string{"/host/key"}).
				WithKnownHostsPaths([]string{"/host/known_hosts"}),
			config: sshremote.NewConfig().
				WithPrivateKeysPaths([]string{"/config/key"}).
				WithKnownHostsPaths([]string{"/config/known_hosts"}),
			want: []string{
				"-i", "/host/key",
				"-i", "/config/key",
				"-p", "2222",
				"-o", "UserKnownHostsFile=/host/known_hosts",
			},
		},
		{
			name: "config known hosts fallback",
			host: sshremote.NewHost("example.com"),
			config: sshremote.NewConfig().
				WithKnownHostsPaths([]string{"/config/known_hosts"}),
			want: []string{"-o", "UserKnownHostsFile=/config/known_hosts"},
		},
		{
			name: "multiple known hosts files",
			host: sshremote.NewHost("example.com").
				WithKnownHostsPaths([]string{"/known_hosts", "/known_hosts2"}),
			config: sshremote.NewConfig(),
			want:   []string{"-o", "UserKnownHostsFile=/known_hosts /known_hosts2"},
		},
		{
			name: "proxy command",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithProxyCommand("nc proxy 22"),
			config: sshremote.NewConfig(),
			want:   []string{"-o", "ProxyCommand=nc proxy 22"},
		},
		{
			name: "proxy transport",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithTransport(sshremote.JumpTransport{Host: sshremote.NewHost("proxy").WithUser("deploy")}),
			config: sshremote.NewConfig(),
			want:   []string{"-o", "ProxyCommand=ssh deploy@proxy nc %h %p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sshremote.NewHostConfig(tt.host, tt.config).SSHArgs()
			if !slices.Equal(got, tt.want) {
				t.Errorf("SSHArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestHostConfigSSHCommand(t *testing.T) {
	tests := []struct {
		name   string
		host   *sshremote.Host
		config *sshremote.Config
		want   string
	}{
		{
			name:   "user and host",
			host:   sshremote.NewHost("example.com").WithUser("deploy"),
			config: sshremote.NewConfig(),
			want:   "ssh deploy@example.com",
		},
		{
			name: "private keys",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithPrivateKeysPaths([]string{"/host/key"}),
			config: sshremote.NewConfig().WithPrivateKeysPaths([]string{"/config/key"}),
			want:   "ssh deploy@example.com -i /host/key -i /config/key",
		},
		{
			name: "port and known hosts",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithPort(2222).
				WithKnownHostsPaths([]string{"/known_hosts"}),
			config: sshremote.NewConfig(),
			want:   "ssh deploy@example.com -p 2222 -o UserKnownHostsFile=/known_hosts",
		},
		{
			name: "multiple known hosts files",
			host: sshremote.NewHost("example.com").
				WithUser("deploy").
				WithKnownHostsPaths([]string{"/known_hosts", "/known_hosts2"}),
			config: sshremote.NewConfig(),
			want:   "ssh deploy@example.com -o 'UserKnownHostsFile=/known_hosts /known_hosts2'",
		},
		{
			name:   "ipv6",
			host:   sshremote.NewHost("::1").WithUser("deploy"),
			config: sshremote.NewConfig(),
			want:   "ssh deploy@\\[::1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sshremote.NewHostConfig(tt.host, tt.config).SSHCommand(); got != tt.want {
				t.Errorf("SSHCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostConfigProxyCommand(t *testing.T) {
	config := sshremote.NewConfig()

	h := sshremote.NewHost("example.com").
		WithUser("deploy").
		WithProxyCommand("nc proxy 22")
	if got := sshremote.NewHostConfig(h, config).ProxyCommand(); got != "nc proxy 22" {
		t.Errorf("ProxyCommand() = %q, want %q", got, "nc proxy 22")
	}

	proxy := sshremote.NewHost("proxy").WithUser("deploy")
	h = sshremote.NewHost("example.com").
		WithUser("deploy").
		WithTransport(sshremote.JumpTransport{Host: proxy})

	want := sshremote.NewHostConfig(proxy, config).SSHCommand() + " nc %h %p"
	if got := sshremote.NewHostConfig(h, config).ProxyCommand(); got != want {
		t.Errorf("ProxyCommand() = %q, want %q", got, want)
	}

	h = sshremote.NewHost("example.com").
		WithUser("deploy").
		WithTransport(sshremote.DialTransport{Addr: "proxy.example.com:2222"})
	if got := sshremote.NewHostConfig(h, config).ProxyCommand(); got != "nc proxy.example.com 2222" {
		t.Errorf("ProxyCommand() = %q, want %q", got, "nc proxy.example.com 2222")
	}

	forwardHost := sshremote.NewHost("jump").WithUser("deploy")
	h = sshremote.NewHost("example.com").
		WithUser("deploy").
		WithTransport(sshremote.ForwardTransport{Host: forwardHost, Addr: "other.example.com:22"})
	if got, want := sshremote.NewHostConfig(h, config).ProxyCommand(), sshremote.NewHostConfig(forwardHost, config).SSHCommand()+" nc other.example.com 22"; got != want {
		t.Errorf("ProxyCommand() = %q, want %q", got, want)
	}

	sessionHost := sshremote.NewHost("jump").WithUser("deploy")
	h = sshremote.NewHost("example.com").
		WithUser("deploy").
		WithTransport(sshremote.SessionCommandTransport{Host: sessionHost, Command: "socat - TCP:%h:%p"})
	if got, want := sshremote.NewHostConfig(h, config).ProxyCommand(), sshremote.NewHostConfig(sessionHost, config).SSHCommand()+" socat - TCP:%h:%p"; got != want {
		t.Errorf("ProxyCommand() = %q, want %q", got, want)
	}

	h = sshremote.NewHost("example.com").
		WithUser("deploy").
		WithTransport(sshremote.StdioTransport{})
	if got := sshremote.NewHostConfig(h, config).ProxyCommand(); got != "-" {
		t.Errorf("ProxyCommand() = %q, want %q", got, "-")
	}
}
