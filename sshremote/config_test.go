package sshremote_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric/sshremote"
)

func TestNewDefaultConfig(t *testing.T) {
	config, err := sshremote.NewDefaultConfig()
	if err != nil {
		t.Fatalf("NewDefaultConfig() error: %v", err)
	}

	if config.StrictHostKeyChecking() != sshremote.StrictHostKeyCheckingYes {
		t.Errorf("StrictHostKeyChecking = %q, want %q", config.StrictHostKeyChecking(), sshremote.StrictHostKeyCheckingYes)
	}

	if config.SSHConfigPath() == "" || len(config.KnownHostsPaths()) == 0 {
		t.Error("SSHConfigPath/KnownHostsPaths should be set")
	}

	if config.ConnectionTimeout() != sshremote.DefaultConnectionTimeout {
		t.Errorf("ConnectionTimeout = %s, want %s", config.ConnectionTimeout(), sshremote.DefaultConnectionTimeout)
	}

	if config.SessionOpenTimeout() != sshremote.DefaultSessionOpenTimeout {
		t.Errorf("SessionOpenTimeout = %s, want %s", config.SessionOpenTimeout(), sshremote.DefaultSessionOpenTimeout)
	}
}

func TestNewDefaultConfigPrivateKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	for _, name := range []string{"id_rsa", "id_ed25519"} {
		if err := os.WriteFile(filepath.Join(sshDir, name), []byte("key"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", name, err)
		}
	}

	// A directory with a default key name is not a usable private key.
	if err := os.Mkdir(filepath.Join(sshDir, "id_ecdsa"), 0o700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	config, err := sshremote.NewDefaultConfig()
	if err != nil {
		t.Fatalf("NewDefaultConfig() error: %v", err)
	}

	want := []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_rsa"),
	}
	if got := config.PrivateKeysPaths(); !slices.Equal(got, want) {
		t.Errorf("PrivateKeysPaths() = %v, want %v", got, want)
	}
}

func TestNewConfigDefaults(t *testing.T) {
	config := sshremote.NewConfig()

	if config.IdleConnections() != sshremote.DefaultIdleConnections {
		t.Errorf("IdleConnections = %d, want %d", config.IdleConnections(), sshremote.DefaultIdleConnections)
	}
	if config.MaxConnections() != sshremote.DefaultMaxConnections {
		t.Errorf("MaxConnections = %d, want %d", config.MaxConnections(), sshremote.DefaultMaxConnections)
	}
}

func TestConfigSigners(t *testing.T) {
	signer := newTestSigner(t)

	config := sshremote.NewConfig().WithSigners(signer)
	if got := config.Signers(); len(got) != 1 {
		t.Fatalf("len(Signers()) = %d, want 1", len(got))
	}

	if !config.Equal(config.Clone()) {
		t.Error("Equal() = false for a clone, want true")
	}

	if config.Equal(sshremote.NewConfig()) {
		t.Error("Equal() = true without the signer, want false")
	}

	signers := config.Signers()
	signers[0] = nil

	if config.Signers()[0] == nil {
		t.Error("Signers() returned the live slice")
	}
}

func TestConfigClone(t *testing.T) {
	config := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/key/a", "/key/b"}).
		WithSSHConfigPath("/ssh/config").
		WithKnownHostsPaths([]string{"/ssh/known_hosts"}).
		WithAgentSocket("/agent.sock").
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes).
		WithConnectionTimeout(5 * time.Second).
		WithSessionOpenTimeout(5 * time.Second).
		WithIdleConnections(2).
		WithMaxConnections(10).
		WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
			return nil, nil
		})

	clone := config.Clone()

	if !config.Equal(clone) {
		t.Error("Equal() = false for a clone, want true")
	}

	changed := config.WithSessionOpenTimeout(time.Minute)
	if config.Equal(changed) {
		t.Error("Equal() = true after changing SessionOpenTimeout, want false")
	}

	changed = config.WithMaxConnections(20)
	if config.Equal(changed) {
		t.Error("Equal() = true after changing MaxConnections, want false")
	}

	changed = config.WithPrivateKeysPaths([]string{"/other"})
	if config.Equal(changed) {
		t.Error("Equal() = true after changing PrivateKeysPaths, want false")
	}
	paths := config.PrivateKeysPaths()
	paths[0] = "/mutated"
	if got := config.PrivateKeysPaths()[0]; got != "/key/a" {
		t.Errorf("PrivateKeysPaths getter exposed the internal slice: %v", config.PrivateKeysPaths())
	}

	if got := config.PrivateKeysPaths()[0]; got != "/key/a" {
		t.Errorf("WithPrivateKeysPaths changed the original: %v", config.PrivateKeysPaths())
	}

	changed = config.WithKnownHostsPaths([]string{"/other_known_hosts"})
	if config.Equal(changed) {
		t.Error("Equal() = true after changing KnownHostsPaths, want false")
	}
	if got := config.KnownHostsPaths()[0]; got != "/ssh/known_hosts" {
		t.Errorf("WithKnownHostsPaths changed the original: %v", config.KnownHostsPaths())
	}

	changed = config.WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		return nil, nil
	})
	if config.Equal(changed) {
		t.Error("Equal() = true after WithHostKeyCallback, want false")
	}
}

func TestConfigEqual(t *testing.T) {
	base := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/key/a"}).
		WithSSHConfigPath("/ssh/config").
		WithKnownHostsPaths([]string{"/ssh/known_hosts"}).
		WithAgentSocket("/agent.sock").
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes).
		WithConnectionTimeout(5 * time.Second)

	if (*sshremote.Config)(nil).Equal(base) {
		t.Error("nil.Equal(non-nil) = true, want false")
	}

	if base.Equal(nil) {
		t.Error("non-nil.Equal(nil) = true, want false")
	}

	if !(*sshremote.Config)(nil).Equal(nil) {
		t.Error("nil.Equal(nil) = false, want true")
	}

	if !base.Equal(base.Clone()) {
		t.Error("Equal() = false for an equal config, want true")
	}

	other := base.WithConnectionTimeout(time.Minute)
	if base.Equal(other) {
		t.Error("Equal() = true for a config with a different ConnectionTimeout, want false")
	}

	other = base.WithKnownHostsPaths([]string{"/ssh/other_known_hosts"})
	if base.Equal(other) {
		t.Error("Equal() = true for a config with different known hosts, want false")
	}

	withCallback := base.WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		return nil, nil
	})

	if base.Equal(withCallback) {
		t.Error("Equal() = true after WithHostKeyCallback, want false")
	}

	if !withCallback.Equal(withCallback.Clone()) {
		t.Error("Equal() = false for a clone of a config with a callback, want true")
	}

	otherWithCallback := base.WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		return nil, nil
	})

	if withCallback.Equal(otherWithCallback) {
		t.Error("Equal() = true for two independently set callbacks, want false")
	}

	if !base.Equal(withCallback.WithHostKeyCallback(nil)) {
		t.Error("WithHostKeyCallback(nil) should compare equal to a config without a callback")
	}
}

const resolveHostSSHConfig = `
Host example
    HostName example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_example
    StrictHostKeyChecking no
`

func TestConfigResolveHost(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	sshConfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfigPath, []byte(resolveHostSSHConfig), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	config := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/default/key"}).
		WithSSHConfigPath(sshConfigPath).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes)

	host, err := config.ResolveHost("example")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.Hostname() != "example.com" {
		t.Errorf("Hostname = %q, want %q", host.Hostname(), "example.com")
	}
	if host.User() != "deploy" {
		t.Errorf("User = %q, want %q", host.User(), "deploy")
	}
	if host.Port() != 2222 {
		t.Errorf("Port = %d, want %d", host.Port(), 2222)
	}
	if host.StrictHostKeyChecking() != sshremote.StrictHostKeyCheckingNo {
		t.Errorf("StrictHostKeyChecking = %q, want %q", host.StrictHostKeyChecking(), sshremote.StrictHostKeyCheckingNo)
	}

	wantKeys := []string{filepath.Join(home, ".ssh", "id_example"), "/default/key"}
	if !slices.Equal(sshremote.NewHostConfig(host, config).PrivateKeysPaths(), wantKeys) {
		t.Errorf("PrivateKeysPaths = %v, want %v", sshremote.NewHostConfig(host, config).PrivateKeysPaths(), wantKeys)
	}
}

func TestConfigResolveHostContent(t *testing.T) {
	const content = `Host example
  HostName example.com
  User deploy
  Port 2222
  IdentityFile /keys/id_example
  StrictHostKeyChecking no
`

	raw := []byte(content)
	config := sshremote.NewConfig().
		WithSSHConfigPath("/unused/config").
		WithSSHConfigContent(raw)

	if got := config.SSHConfigPath(); got != "" {
		t.Errorf("SSHConfigPath() = %q, want empty", got)
	}
	if got := config.SSHConfigContent(); !bytes.Equal(got, []byte(content)) {
		t.Errorf("SSHConfigContent() = %q, want %q", got, content)
	}

	raw[0] = 'X'
	if got := config.SSHConfigContent(); !bytes.Equal(got, []byte(content)) {
		t.Errorf("SSHConfigContent() changed after mutating the input: %q", got)
	}

	got := config.SSHConfigContent()
	got[0] = 'Y'
	if got := config.SSHConfigContent(); !bytes.Equal(got, []byte(content)) {
		t.Errorf("SSHConfigContent() returned the internal slice: %q", got)
	}

	host, err := config.ResolveHost("example")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}
	if host.Hostname() != "example.com" {
		t.Errorf("Hostname = %q, want %q", host.Hostname(), "example.com")
	}
	if host.User() != "deploy" {
		t.Errorf("User = %q, want %q", host.User(), "deploy")
	}
	if host.Port() != 2222 {
		t.Errorf("Port = %d, want %d", host.Port(), 2222)
	}
	if host.StrictHostKeyChecking() != sshremote.StrictHostKeyCheckingNo {
		t.Errorf("StrictHostKeyChecking = %q, want %q", host.StrictHostKeyChecking(), sshremote.StrictHostKeyCheckingNo)
	}
	if keys := host.PrivateKeysPaths(); !slices.Contains(keys, "/keys/id_example") {
		t.Errorf("PrivateKeysPaths() = %v, want /keys/id_example", keys)
	}

	if !config.Equal(config.Clone()) {
		t.Error("Equal() = false for a clone, want true")
	}
	if config.Equal(config.WithSSHConfigContent([]byte("Host other\n"))) {
		t.Error("Equal() = true after changing ssh_config content, want false")
	}

	pathConfig := config.WithSSHConfigPath("/new/config")
	if got := pathConfig.SSHConfigContent(); got != nil {
		t.Errorf("SSHConfigContent() after WithSSHConfigPath = %q, want nil", got)
	}
	if got, want := pathConfig.SSHConfigPath(), "/new/config"; got != want {
		t.Errorf("SSHConfigPath() = %q, want %q", got, want)
	}
}

func TestConfigResolveHostAcceptNew(t *testing.T) {
	const sshConfig = `
Host example
    HostName example.com
    StrictHostKeyChecking accept-new
`

	host, err := resolveHostWithConfig(t, sshConfig, "example")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.StrictHostKeyChecking() != sshremote.StrictHostKeyCheckingAcceptNew {
		t.Errorf("StrictHostKeyChecking = %q, want %q", host.StrictHostKeyChecking(), sshremote.StrictHostKeyCheckingAcceptNew)
	}
}

func TestConfigResolveHostMultipleValues(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	const sshConfig = `
Host example
    HostName example.com
    IdentityFile ~/.ssh/id_example1
    IdentityFile ~/.ssh/id_example2
    UserKnownHostsFile ~/.ssh/known_hosts ~/.ssh/known_hosts2

Host *
    IdentityFile ~/.ssh/id_global
    UserKnownHostsFile ~/.ssh/global_known_hosts
`

	host, err := resolveHostWithConfig(t, sshConfig, "example")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	wantKeys := []string{
		filepath.Join(home, ".ssh", "id_example1"),
		filepath.Join(home, ".ssh", "id_example2"),
		filepath.Join(home, ".ssh", "id_global"),
	}
	if !slices.Equal(host.PrivateKeysPaths(), wantKeys) {
		t.Errorf("PrivateKeysPaths = %v, want %v", host.PrivateKeysPaths(), wantKeys)
	}

	// UserKnownHostsFile takes the first matching directive, which may list
	// several whitespace-separated files.
	wantKnownHosts := []string{
		filepath.Join(home, ".ssh", "known_hosts"),
		filepath.Join(home, ".ssh", "known_hosts2"),
	}
	if !slices.Equal(host.KnownHostsPaths(), wantKnownHosts) {
		t.Errorf("KnownHostsPaths = %v, want %v", host.KnownHostsPaths(), wantKnownHosts)
	}
}

func resolveHostWithConfig(t *testing.T, sshConfig string, hostStr string) (*sshremote.Host, error) {
	t.Helper()

	sshConfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	config := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/default/key"}).
		WithSSHConfigPath(sshConfigPath).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes)

	return config.ResolveHost(hostStr)
}

func jumpHost(t *testing.T, tr sshremote.Transport) *sshremote.Host {
	t.Helper()

	jump, ok := tr.(sshremote.JumpTransport)
	if !ok {
		t.Fatalf("transport = %T, want JumpTransport", tr)
	}
	if jump.Host == nil {
		t.Fatal("JumpTransport.Host = nil")
	}

	return jump.Host
}

const proxyJumpSSHConfig = `
Host jump
    HostName jump.example.com
    User jumpuser
    Port 2200
    IdentityFile ~/.ssh/id_jump

Host target
    HostName target.example.com
    ProxyJump jump
`

func TestConfigResolveHostProxyJump(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	host, err := resolveHostWithConfig(t, proxyJumpSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.Hostname() != "target.example.com" {
		t.Errorf("Hostname = %q, want %q", host.Hostname(), "target.example.com")
	}

	jump := jumpHost(t, host.Transport())

	if jump.Hostname() != "jump.example.com" {
		t.Errorf("jump.Hostname() = %q, want %q", jump.Hostname(), "jump.example.com")
	}
	if jump.User() != "jumpuser" {
		t.Errorf("jump.User() = %q, want %q", jump.User(), "jumpuser")
	}
	if jump.Port() != 2200 {
		t.Errorf("jump.Port() = %d, want %d", jump.Port(), 2200)
	}

	wantKeys := []string{filepath.Join(home, ".ssh", "id_jump")}
	if !slices.Equal(jump.PrivateKeysPaths(), wantKeys) {
		t.Errorf("jump.PrivateKeysPaths() = %v, want %v", jump.PrivateKeysPaths(), wantKeys)
	}
}

const proxyJumpChainSSHConfig = `
Host a
    HostName a.example.com

Host b
    HostName b.example.com

Host target
    ProxyJump a,b
`

func TestConfigResolveHostProxyJumpChain(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpChainSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	b := jumpHost(t, host.Transport())
	if b.Hostname() != "b.example.com" {
		t.Fatalf("jump host = %q, want b.example.com", b.Hostname())
	}

	a := jumpHost(t, b.Transport())
	if a.Hostname() != "a.example.com" {
		t.Fatalf("jump host = %q, want a.example.com", a.Hostname())
	}

	if a.Transport() != nil {
		t.Errorf("a.Transport() = %+v, want nil", a.Transport())
	}
}

const proxyJumpNestedSSHConfig = `
Host c
    HostName c.example.com

Host a
    HostName a.example.com
    ProxyJump c

Host b
    HostName b.example.com

Host target
    ProxyJump a,b
`

func TestConfigResolveHostProxyJumpNested(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpNestedSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	b := jumpHost(t, host.Transport())
	if b.Hostname() != "b.example.com" {
		t.Fatalf("jump host = %q, want b.example.com", b.Hostname())
	}

	a := jumpHost(t, b.Transport())
	if a.Hostname() != "a.example.com" {
		t.Fatalf("jump host = %q, want a.example.com", a.Hostname())
	}

	c := jumpHost(t, a.Transport())
	if c.Hostname() != "c.example.com" {
		t.Fatalf("jump host = %q, want c.example.com", c.Hostname())
	}
}

const proxyJumpSuppressHopSSHConfig = `
Host y
    HostName y.example.com

Host a
    HostName a.example.com

Host b
    HostName b.example.com
    ProxyJump y

Host target
    ProxyJump a,b
`

func TestConfigResolveHostProxyJumpSuppressesHopProxy(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpSuppressHopSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	b := jumpHost(t, host.Transport())
	if b.Hostname() != "b.example.com" {
		t.Fatalf("jump host = %q, want b.example.com", b.Hostname())
	}

	a := jumpHost(t, b.Transport())
	if a.Hostname() != "a.example.com" {
		t.Fatalf("jump host = %q, want a.example.com (b's own ProxyJump suppressed)", a.Hostname())
	}
}

const proxyJumpSingleHopChainSSHConfig = `
Host c
    HostName c.example.com

Host a
    HostName a.example.com
    ProxyJump c

Host target
    ProxyJump a
`

func TestConfigResolveHostProxyJumpSingleHopKeepsOwnChain(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpSingleHopChainSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	a := jumpHost(t, host.Transport())
	if a.Hostname() != "a.example.com" {
		t.Fatalf("jump host = %q, want a.example.com", a.Hostname())
	}

	c := jumpHost(t, a.Transport())
	if c.Hostname() != "c.example.com" {
		t.Fatalf("jump host = %q, want c.example.com (a's own ProxyJump kept)", c.Hostname())
	}
}

const proxyJumpUserPortSSHConfig = `
Host target
    ProxyJump deploy@jump.example.com:2222
`

func TestConfigResolveHostProxyJumpUserPort(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpUserPortSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	jump := jumpHost(t, host.Transport())

	if jump.Hostname() != "jump.example.com" {
		t.Errorf("jump.Hostname() = %q, want %q", jump.Hostname(), "jump.example.com")
	}
	if jump.User() != "deploy" {
		t.Errorf("jump.User() = %q, want %q", jump.User(), "deploy")
	}
	if jump.Port() != 2222 {
		t.Errorf("jump.Port() = %d, want %d", jump.Port(), 2222)
	}
}

const proxyJumpNoneSSHConfig = `
Host target
    ProxyJump none
`

func TestConfigResolveHostProxyJumpNone(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyJumpNoneSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.Transport() != nil {
		t.Errorf("Transport = %+v, want nil", host.Transport())
	}
}

const proxyCommandPrecedenceSSHConfig = `
Host jump
    HostName jump.example.com

Host target
    ProxyCommand ssh jump
    ProxyJump other
`

func TestConfigResolveHostProxyCommandPrecedence(t *testing.T) {
	host, err := resolveHostWithConfig(t, proxyCommandPrecedenceSSHConfig, "target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.ProxyCommand() != "ssh jump" {
		t.Errorf("ProxyCommand = %q, want %q", host.ProxyCommand(), "ssh jump")
	}

	jump := jumpHost(t, host.Transport())
	if jump.Hostname() != "jump.example.com" {
		t.Errorf("jump host = %q, want the ProxyCommand host (ProxyJump ignored)", jump.Hostname())
	}
}

func TestConfigResolveHostProxyCommand(t *testing.T) {
	tests := []struct {
		name         string
		proxyCommand string
		jump         string
		user         string
		port         int
		dial         string
		forward      string
		session      string
		stdio        bool
		command      string
	}{
		{name: "bare", proxyCommand: "ssh jump", jump: "jump"},
		{name: "user at host port", proxyCommand: "ssh deploy@jump.example.com:2222", jump: "jump.example.com", user: "deploy", port: 2222},
		{name: "forward", proxyCommand: "ssh -W %h:%p jump", jump: "jump"},
		{name: "forward other", proxyCommand: "ssh -W other.example.com:22 jump", jump: "jump", forward: "other.example.com:22"},
		{name: "nc trailer", proxyCommand: "ssh jump nc %h %p", jump: "jump"},
		{name: "nc forward", proxyCommand: "ssh jump nc other.example.com 22", jump: "jump", forward: "other.example.com:22"},
		{name: "flags", proxyCommand: "ssh -l user -p 2200 jump", jump: "jump", user: "user", port: 2200},
		{name: "unsupported flag", proxyCommand: "ssh -o X=Y jump", command: "ssh -o X=Y jump"},
		{name: "session", proxyCommand: "ssh jump socat - TCP:%h:%p", jump: "jump", session: "socat - TCP:%h:%p"},
		{name: "session nc bad port", proxyCommand: "ssh jump nc other notaport", jump: "jump", session: "nc other notaport"},
		{name: "stdio", proxyCommand: "-", stdio: true},
		{name: "nc", proxyCommand: "nc proxy 22", dial: "proxy:22"},
		{name: "nc flags", proxyCommand: "nc -w 5 other.example.com 2222", dial: "other.example.com:2222"},
		{name: "nc target", proxyCommand: "nc %h %p"},
		{name: "nc bad port", proxyCommand: "nc other notaport", command: "nc other notaport"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sshConfig := "Host target\n    ProxyCommand " + tt.proxyCommand + "\n"

			host, err := resolveHostWithConfig(t, sshConfig, "target")
			if err != nil {
				t.Fatalf("ResolveHost() error: %v", err)
			}

			if host.ProxyCommand() != tt.proxyCommand {
				t.Errorf("ProxyCommand = %q, want %q (raw preserved)", host.ProxyCommand(), tt.proxyCommand)
			}

			if tt.stdio {
				if _, ok := host.Transport().(sshremote.StdioTransport); !ok {
					t.Fatalf("Transport = %T, want StdioTransport", host.Transport())
				}
				return
			}

			if tt.command != "" {
				commandTransport, ok := host.Transport().(sshremote.CommandTransport)
				if !ok {
					t.Fatalf("Transport = %T, want CommandTransport", host.Transport())
				}
				if commandTransport.Command != tt.command {
					t.Errorf("CommandTransport.Command = %q, want %q", commandTransport.Command, tt.command)
				}
				return
			}

			if tt.dial != "" {
				dialTransport, ok := host.Transport().(sshremote.DialTransport)
				if !ok {
					t.Fatalf("Transport = %T, want DialTransport", host.Transport())
				}
				if dialTransport.Addr != tt.dial {
					t.Errorf("DialTransport.Addr = %q, want %q", dialTransport.Addr, tt.dial)
				}
				return
			}

			if tt.forward != "" {
				forwardTransport, ok := host.Transport().(sshremote.ForwardTransport)
				if !ok {
					t.Fatalf("Transport = %T, want ForwardTransport", host.Transport())
				}
				if forwardTransport.Host == nil || forwardTransport.Host.Hostname() != tt.jump {
					t.Errorf("ForwardTransport.Host = %+v, want %q", forwardTransport.Host, tt.jump)
				}
				if forwardTransport.Addr != tt.forward {
					t.Errorf("ForwardTransport.Addr = %q, want %q", forwardTransport.Addr, tt.forward)
				}
				return
			}

			if tt.session != "" {
				sessionTransport, ok := host.Transport().(sshremote.SessionCommandTransport)
				if !ok {
					t.Fatalf("Transport = %T, want SessionCommandTransport", host.Transport())
				}
				if sessionTransport.Host == nil || sessionTransport.Host.Hostname() != tt.jump {
					t.Errorf("SessionCommandTransport.Host = %+v, want %q", sessionTransport.Host, tt.jump)
				}
				if sessionTransport.Command != tt.session {
					t.Errorf("SessionCommandTransport.Command = %q, want %q", sessionTransport.Command, tt.session)
				}
				return
			}

			if tt.jump == "" {
				if host.Transport() != nil {
					t.Errorf("Transport = %+v, want nil", host.Transport())
				}
				return
			}

			jump := jumpHost(t, host.Transport())
			if jump.Hostname() != tt.jump {
				t.Errorf("jump.Hostname() = %q, want %q", jump.Hostname(), tt.jump)
			}
			if tt.user != "" && jump.User() != tt.user {
				t.Errorf("jump.User() = %q, want %q", jump.User(), tt.user)
			}
			if tt.port != 0 && jump.Port() != tt.port {
				t.Errorf("jump.Port() = %d, want %d", jump.Port(), tt.port)
			}
		})
	}
}

func TestConfigResolveHostProxyDirectiveOrder(t *testing.T) {
	tests := []struct {
		name      string
		sshConfig string
		jump      string
	}{
		{
			name: "proxycommand first",
			sshConfig: "Host jump\n" +
				"    HostName jump.example.com\n" +
				"\n" +
				"Host target\n" +
				"    ProxyCommand ssh jump\n" +
				"    ProxyJump other\n",
			jump: "jump.example.com",
		},
		{
			name: "proxyjump first",
			sshConfig: "Host a\n" +
				"    HostName a.example.com\n" +
				"\n" +
				"Host target\n" +
				"    ProxyJump a\n" +
				"    ProxyCommand ssh jump\n",
			jump: "a.example.com",
		},
		{
			name: "across blocks",
			sshConfig: "Host target\n" +
				"    ProxyJump a\n" +
				"\n" +
				"Host a\n" +
				"    HostName a.example.com\n" +
				"\n" +
				"Host jump\n" +
				"    HostName jump.example.com\n" +
				"\n" +
				"Host target\n" +
				"    ProxyCommand ssh jump\n",
			jump: "a.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := resolveHostWithConfig(t, tt.sshConfig, "target")
			if err != nil {
				t.Fatalf("ResolveHost() error: %v", err)
			}

			jump := jumpHost(t, host.Transport())
			if jump.Hostname() != tt.jump {
				t.Errorf("jump host = %q, want %q", jump.Hostname(), tt.jump)
			}
		})
	}
}

const proxyJumpLoopSSHConfig = `
Host a
    ProxyJump a
`

const proxyJumpMutualLoopSSHConfig = `
Host a
    ProxyJump b

Host b
    ProxyJump a
`

func TestConfigResolveHostProxyJumpLoop(t *testing.T) {
	tests := []struct {
		name      string
		sshConfig string
	}{
		{
			name:      "self loop",
			sshConfig: proxyJumpLoopSSHConfig,
		},
		{
			name:      "mutual loop",
			sshConfig: proxyJumpMutualLoopSSHConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := resolveHostWithConfig(t, tt.sshConfig, "a"); err == nil {
				t.Fatal("ResolveHost() = nil error, want a proxy chain loop error")
			}
		})
	}
}

func TestConfigResolveHostSSHConfigGlobalDefaults(t *testing.T) {
	const sshConfig = `
IdentityFile /keys/global
UserKnownHostsFile /known/global
StrictHostKeyChecking no

Host server1.example.com
    User alice
`

	host, err := resolveHostWithConfig(t, sshConfig, "server1.example.com")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if got, want := host.PrivateKeysPaths(), []string{"/keys/global"}; !slices.Equal(got, want) {
		t.Errorf("PrivateKeysPaths() = %v, want %v", got, want)
	}
	if got, want := host.KnownHostsPaths(), []string{"/known/global"}; !slices.Equal(got, want) {
		t.Errorf("KnownHostsPaths() = %v, want %v", got, want)
	}
	if got := host.StrictHostKeyChecking(); got != sshremote.StrictHostKeyCheckingNo {
		t.Errorf("StrictHostKeyChecking() = %q, want %q", got, sshremote.StrictHostKeyCheckingNo)
	}
	if got := host.User(); got != "alice" {
		t.Errorf("User() = %q, want %q", got, "alice")
	}
}

func TestConfigResolveHostSSHConfigIdentityFileOrder(t *testing.T) {
	const sshConfig = `
IdentityFile /keys/global

Host *.internal
    IdentityFile /keys/wildcard

Host server1.internal
    IdentityFile /keys/exact
`

	host, err := resolveHostWithConfig(t, sshConfig, "server1.internal")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	want := []string{"/keys/global", "/keys/wildcard", "/keys/exact"}
	if got := host.PrivateKeysPaths(); !slices.Equal(got, want) {
		t.Errorf("PrivateKeysPaths() = %v, want %v", got, want)
	}
}

func TestConfigResolveHostSSHConfigFirstMatch(t *testing.T) {
	tests := []struct {
		name      string
		sshConfig string
		host      string
		want      sshremote.StrictHostKeyChecking
	}{
		{
			name: "global before host",
			sshConfig: `
StrictHostKeyChecking no

Host example
    StrictHostKeyChecking yes
`,
			host: "example",
			want: sshremote.StrictHostKeyCheckingNo,
		},
		{
			name: "wildcard before exact",
			sshConfig: `
Host *.internal
    StrictHostKeyChecking accept-new

Host server.internal
    StrictHostKeyChecking yes
`,
			host: "server.internal",
			want: sshremote.StrictHostKeyCheckingAcceptNew,
		},
		{
			name: "exact before wildcard",
			sshConfig: `
Host server.internal
    StrictHostKeyChecking yes

Host *.internal
    StrictHostKeyChecking no
`,
			host: "server.internal",
			want: sshremote.StrictHostKeyCheckingYes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := resolveHostWithConfig(t, tt.sshConfig, tt.host)
			if err != nil {
				t.Fatalf("ResolveHost() error: %v", err)
			}

			if got := host.StrictHostKeyChecking(); got != tt.want {
				t.Errorf("StrictHostKeyChecking() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigResolveHostSSHConfigKnownHostsFirstMatch(t *testing.T) {
	tests := []struct {
		name      string
		sshConfig string
		want      []string
	}{
		{
			name: "global before host",
			sshConfig: `
UserKnownHostsFile /known/global

Host example
    UserKnownHostsFile /known/host
`,
			want: []string{"/known/global"},
		},
		{
			name: "host before global",
			sshConfig: `
Host example
    UserKnownHostsFile /known/host

Host *
    UserKnownHostsFile /known/global
`,
			want: []string{"/known/host"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := resolveHostWithConfig(t, tt.sshConfig, "example")
			if err != nil {
				t.Fatalf("ResolveHost() error: %v", err)
			}

			if got := host.KnownHostsPaths(); !slices.Equal(got, tt.want) {
				t.Errorf("KnownHostsPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigResolveHostDoesNotCopyConfigDefaults(t *testing.T) {
	const sshConfig = `
Host example
    HostName example.com
`

	sshConfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	config := sshremote.NewConfig().
		WithPrivateKeysPaths([]string{"/config/key"}).
		WithKnownHostsPaths([]string{"/config/known_hosts"}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew).
		WithSSHConfigPath(sshConfigPath)

	host, err := config.ResolveHost("example")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if got := host.PrivateKeysPaths(); len(got) != 0 {
		t.Errorf("PrivateKeysPaths() = %v, want empty", got)
	}
	if got := host.KnownHostsPaths(); len(got) != 0 {
		t.Errorf("KnownHostsPaths() = %v, want empty", got)
	}
	if got := host.StrictHostKeyChecking(); got != "" {
		t.Errorf("StrictHostKeyChecking() = %q, want empty", got)
	}

	hc := sshremote.NewHostConfig(host, config)
	if got, want := hc.PrivateKeysPaths(), []string{"/config/key"}; !slices.Equal(got, want) {
		t.Errorf("HostConfig.PrivateKeysPaths() = %v, want %v", got, want)
	}
	if got, want := hc.KnownHostsPaths(), []string{"/config/known_hosts"}; !slices.Equal(got, want) {
		t.Errorf("HostConfig.KnownHostsPaths() = %v, want %v", got, want)
	}
	if got := hc.StrictHostKeyChecking(); got != sshremote.StrictHostKeyCheckingAcceptNew {
		t.Errorf("HostConfig.StrictHostKeyChecking() = %q, want %q", got, sshremote.StrictHostKeyCheckingAcceptNew)
	}
}
