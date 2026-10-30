package sshremote

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
)

// DefaultConnectionTimeout is the default TCP dial and SSH handshake timeout.
const DefaultConnectionTimeout = 30 * time.Second

// DefaultSessionOpenTimeout is the default timeout for opening an SSH session.
const DefaultSessionOpenTimeout = DefaultConnectionTimeout

// DefaultSSHPrivateKeyFiles are the private key filenames in ~/.ssh that
// NewDefaultConfig looks up, in the order they are offered to the server.
// Legacy DSA, security-key (sk-*) and XMSS keys are omitted: x/crypto/ssh
// cannot use them, and a key it fails to parse aborts the whole connection.
var DefaultSSHPrivateKeyFiles = []string{
	"id_ed25519",
	"id_ecdsa",
	"id_rsa",
}

// hostKeyCallbackHolder carries a host key callback. Its pointer identifies the
// callback version: WithHostKeyCallback allocates a new holder and Clone copies
// the pointer.
type hostKeyCallbackHolder struct {
	fn func(hostConfig HostConfig) (ssh.HostKeyCallback, error)
}

// Config is the immutable remote connection configuration: identity, host
// resolution, host key checking and pooling limits. Its pointer is part of the
// Remote identity, so a different config selects a different Remote.
type Config struct {
	privateKeysPaths      []string
	sshSigners            []ssh.Signer
	sshConfigPath         string
	sshConfigContent      []byte
	knownHostsPaths       []string
	agentSocket           string
	strictHostKeyChecking StrictHostKeyChecking
	hostKeyCallback       *hostKeyCallbackHolder
	connectionTimeout     time.Duration
	sessionOpenTimeout    time.Duration
	idleConnections       int
	maxConnections        int

	sshConfigOnce sync.Once
	sshConfig     *ssh_config.Config
	sshConfigErr  error
}

// NewConfig returns a Config with default pooling limits.
func NewConfig() *Config {
	return &Config{
		idleConnections: DefaultIdleConnections,
		maxConnections:  DefaultMaxConnections,
	}
}

// PrivateKeysPaths returns the default private key paths.
func (c *Config) PrivateKeysPaths() []string {
	return slices.Clone(c.privateKeysPaths)
}

// Signers returns the default in-memory signers.
func (c *Config) Signers() []ssh.Signer {
	return cloneSigners(c.sshSigners)
}

// WithSigners returns a copy with the in-memory signers replaced.
func (c *Config) WithSigners(signers ...ssh.Signer) *Config {
	clone := c.Clone()
	clone.sshSigners = cloneSigners(signers)
	return clone
}

// SSHConfigPath returns the ssh_config path, or "" when in-memory content is
// used.
func (c *Config) SSHConfigPath() string {
	return c.sshConfigPath
}

// SSHConfigContent returns a copy of the in-memory ssh_config content, or nil
// when a file path is used.
func (c *Config) SSHConfigContent() []byte {
	return slices.Clone(c.sshConfigContent)
}

// KnownHostsPaths returns the default known_hosts paths.
func (c *Config) KnownHostsPaths() []string {
	return slices.Clone(c.knownHostsPaths)
}

// AgentSocket returns the ssh agent socket.
func (c *Config) AgentSocket() string {
	return c.agentSocket
}

// StrictHostKeyChecking returns the default host key policy.
func (c *Config) StrictHostKeyChecking() StrictHostKeyChecking {
	return c.strictHostKeyChecking
}

// ConnectionTimeout returns the TCP dial and SSH handshake timeout.
func (c *Config) ConnectionTimeout() time.Duration {
	return c.connectionTimeout
}

// SessionOpenTimeout returns the SSH session-open timeout.
func (c *Config) SessionOpenTimeout() time.Duration {
	return c.sessionOpenTimeout
}

// IdleConnections returns the maximum number of idle pooled connections.
func (c *Config) IdleConnections() int {
	return c.idleConnections
}

// MaxConnections returns the maximum number of connections.
func (c *Config) MaxConnections() int {
	return c.maxConnections
}

// WithPrivateKeysPaths returns a copy with the private key paths replaced.
func (c *Config) WithPrivateKeysPaths(paths []string) *Config {
	clone := c.Clone()
	clone.privateKeysPaths = slices.Clone(paths)
	return clone
}

// WithSSHConfigPath returns a copy with the ssh_config path replaced. It clears
// any in-memory content.
func (c *Config) WithSSHConfigPath(path string) *Config {
	clone := c.Clone()
	clone.sshConfigPath = path
	clone.sshConfigContent = nil
	return clone
}

// WithSSHConfigContent returns a copy with the ssh_config content replaced. It
// clears the ssh_config path. The content is copied.
func (c *Config) WithSSHConfigContent(content []byte) *Config {
	clone := c.Clone()
	clone.sshConfigPath = ""
	clone.sshConfigContent = slices.Clone(content)
	return clone
}

// WithKnownHostsPaths returns a copy with the known_hosts paths replaced.
func (c *Config) WithKnownHostsPaths(paths []string) *Config {
	clone := c.Clone()
	clone.knownHostsPaths = slices.Clone(paths)
	return clone
}

// WithAgentSocket returns a copy with the ssh agent socket replaced.
func (c *Config) WithAgentSocket(socket string) *Config {
	clone := c.Clone()
	clone.agentSocket = socket
	return clone
}

// WithStrictHostKeyChecking returns a copy with the host key policy replaced.
func (c *Config) WithStrictHostKeyChecking(policy StrictHostKeyChecking) *Config {
	clone := c.Clone()
	clone.strictHostKeyChecking = policy
	return clone
}

// WithConnectionTimeout returns a copy with the connection timeout replaced.
func (c *Config) WithConnectionTimeout(timeout time.Duration) *Config {
	clone := c.Clone()
	clone.connectionTimeout = timeout
	return clone
}

// WithSessionOpenTimeout returns a copy with the session-open timeout replaced.
func (c *Config) WithSessionOpenTimeout(timeout time.Duration) *Config {
	clone := c.Clone()
	clone.sessionOpenTimeout = timeout
	return clone
}

// WithIdleConnections returns a copy with the idle connection limit replaced.
func (c *Config) WithIdleConnections(limit int) *Config {
	clone := c.Clone()
	clone.idleConnections = limit
	return clone
}

// WithMaxConnections returns a copy with the connection limit replaced.
func (c *Config) WithMaxConnections(limit int) *Config {
	clone := c.Clone()
	clone.maxConnections = limit
	return clone
}

// WithHostKeyCallback returns a copy of c with the host key callback set. The
// callback is used for every connection, independent of the host key policy,
// so a user can disable strict checking and, for example, only log
// fingerprints. A nil callback clears it.
//
// The returned config compares unequal to c and has a different identity.
func (c *Config) WithHostKeyCallback(callback func(hostConfig HostConfig) (ssh.HostKeyCallback, error)) *Config {
	clone := c.Clone()

	if callback == nil {
		clone.hostKeyCallback = nil
		return clone
	}

	clone.hostKeyCallback = &hostKeyCallbackHolder{fn: callback}

	return clone
}

// Clone returns a copy of c. The private key paths are copied and the host key
// callback holder is shared; the cached ssh_config parse is not carried over.
func (c *Config) Clone() *Config {
	return &Config{
		privateKeysPaths:      slices.Clone(c.privateKeysPaths),
		sshSigners:            cloneSigners(c.sshSigners),
		sshConfigPath:         c.sshConfigPath,
		sshConfigContent:      slices.Clone(c.sshConfigContent),
		knownHostsPaths:       slices.Clone(c.knownHostsPaths),
		agentSocket:           c.agentSocket,
		strictHostKeyChecking: c.strictHostKeyChecking,
		hostKeyCallback:       c.hostKeyCallback,
		connectionTimeout:     c.connectionTimeout,
		sessionOpenTimeout:    c.sessionOpenTimeout,
		idleConnections:       c.idleConnections,
		maxConnections:        c.maxConnections,
	}
}

// Equal reports whether c and other carry the same connection settings,
// including the host key callback set with WithHostKeyCallback.
func (c *Config) Equal(other *Config) bool {
	if c == nil || other == nil {
		return c == other
	}

	return slices.Equal(c.privateKeysPaths, other.privateKeysPaths) &&
		signersEqual(c.sshSigners, other.sshSigners) &&
		c.sshConfigPath == other.sshConfigPath &&
		slices.Equal(c.sshConfigContent, other.sshConfigContent) &&
		slices.Equal(c.knownHostsPaths, other.knownHostsPaths) &&
		c.agentSocket == other.agentSocket &&
		c.strictHostKeyChecking == other.strictHostKeyChecking &&
		c.hostKeyCallback == other.hostKeyCallback &&
		c.connectionTimeout == other.connectionTimeout &&
		c.sessionOpenTimeout == other.sessionOpenTimeout &&
		c.idleConnections == other.idleConnections &&
		c.maxConnections == other.maxConnections
}

// NewDefaultConfig builds a Config from the user's environment: ~/.ssh/config,
// ~/.ssh/known_hosts, the default private keys that exist and $SSH_AUTH_SOCK.
func NewDefaultConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(home, ".ssh")

	var privateKeys []string

	for _, name := range DefaultSSHPrivateKeyFiles {
		path := filepath.Join(sshDir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			privateKeys = append(privateKeys, path)
		}
	}

	return &Config{
		privateKeysPaths:      privateKeys,
		sshConfigPath:         filepath.Join(sshDir, "config"),
		knownHostsPaths:       []string{filepath.Join(sshDir, "known_hosts")},
		agentSocket:           os.Getenv("SSH_AUTH_SOCK"),
		strictHostKeyChecking: StrictHostKeyCheckingYes,
		connectionTimeout:     DefaultConnectionTimeout,
		sessionOpenTimeout:    DefaultSessionOpenTimeout,
		idleConnections:       DefaultIdleConnections,
		maxConnections:        DefaultMaxConnections,
	}, nil
}

func (c *Config) sshConfigFile() (*ssh_config.Config, error) {
	c.sshConfigOnce.Do(func() {
		if c.sshConfigContent != nil {
			c.sshConfig, c.sshConfigErr = ssh_config.DecodeBytes(c.sshConfigContent)
			return
		}

		f, err := os.Open(c.sshConfigPath)
		if err != nil {
			// No ssh_config file: no settings to merge.
			return
		}
		defer f.Close()

		c.sshConfig, c.sshConfigErr = ssh_config.Decode(f)
	})

	return c.sshConfig, c.sshConfigErr
}

// ResolveHost parses hostStr and merges matching ssh_config settings.
func (c *Config) ResolveHost(hostStr string) (*Host, error) {
	return c.resolveHost(hostStr, true, map[string]bool{})
}

// resolveHost parses hostStr and merges matching ssh_config settings. When
// followProxy is false, the host's own ProxyCommand/ProxyJump are ignored; this
// is used for ProxyJump hops whose proxy directives are suppressed by an
// explicit ProxyJump chain, matching OpenSSH. seen tracks the aliases being
// resolved to detect proxy chain loops.
func (c *Config) resolveHost(hostStr string, followProxy bool, seen map[string]bool) (*Host, error) {
	h, err := ParseHost(hostStr)
	if err != nil {
		return nil, err
	}

	alias := h.hostname

	if seen[alias] {
		return nil, fmt.Errorf("proxy chain loop via %q", alias)
	}
	seen[alias] = true

	sshConfig, err := c.sshConfigFile()
	if err != nil {
		return nil, err
	}

	if sshConfig == nil {
		return h.finish(), nil
	}

	if hostname, err := sshConfig.Get(alias, "hostname"); err == nil && hostname != "" {
		h.hostname = hostname
	}

	if h.user == "" {
		if user, err := sshConfig.Get(alias, "user"); err == nil && user != "" {
			h.user = user
		}
	}

	if h.port == 0 {
		if portStr, err := sshConfig.Get(alias, "port"); err == nil && portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				h.port = port
			}
		}
	}

	if identityFiles, err := sshConfig.GetAll(alias, "identityfile"); err == nil {
		resolvedIdentityFiles := make([]string, 0, len(identityFiles))

		for _, identityFile := range identityFiles {
			if identityFile == "" {
				continue
			}

			identityFile, err := expandHome(identityFile)
			if err != nil {
				continue
			}

			resolvedIdentityFiles = append(resolvedIdentityFiles, identityFile)
		}

		h.privateKeysPaths = append(resolvedIdentityFiles, h.privateKeysPaths...)
	}

	if strictHostKeyChecking, err := sshConfig.Get(alias, "stricthostkeychecking"); err == nil && strictHostKeyChecking != "" {
		h.strictHostKeyChecking = parseStrictHostKeyChecking(strictHostKeyChecking)
	}

	if userKnownHostsFiles, err := sshConfig.Get(alias, "userknownhostsfile"); err == nil && userKnownHostsFiles != "" {
		for userKnownHostsFile := range strings.FieldsSeq(userKnownHostsFiles) {
			if knownHostsPath, err := expandHome(userKnownHostsFile); err == nil {
				h.knownHostsPaths = append(h.knownHostsPaths, knownHostsPath)
			}
		}
	}

	if !followProxy {
		return h.finish(), nil
	}

	switch key, value := c.proxyDirective(sshConfig, alias); key {
	case "proxycommand":
		h.proxyCommand = value

		transport, ok, err := c.resolveProxyCommand(value, h, seen)
		if err != nil {
			return nil, fmt.Errorf("failed to get host for proxy command: %s: %w", value, err)
		}
		if ok {
			h.transport = transport
		} else {
			h.transport = CommandTransport{Command: value}
		}
	case "proxyjump":
		proxy, err := c.resolveProxyJump(value, seen)
		if err != nil {
			return nil, fmt.Errorf("failed to get host for proxy jump: %s: %w", value, err)
		}

		if proxy != nil {
			h.transport = JumpTransport{Host: proxy}
		}
	}

	return h.finish(), nil
}

// proxyDirective returns the first of ProxyCommand/ProxyJump across the
// matching Host blocks in file order, matching OpenSSH's first-obtained-value
// rule. Include search order is approximate (the library searches included
// files unordered).
func (c *Config) proxyDirective(sshConfig *ssh_config.Config, alias string) (key, value string) {
	for _, host := range sshConfig.Hosts {
		if !host.Matches(alias) {
			continue
		}

		for _, node := range host.Nodes {
			switch t := node.(type) {
			case *ssh_config.KV:
				switch strings.ToLower(t.Key) {
				case "proxycommand":
					return "proxycommand", t.Value
				case "proxyjump":
					return "proxyjump", t.Value
				}
			case *ssh_config.Include:
				if v := t.Get(alias, "proxycommand"); v != "" {
					return "proxycommand", v
				}
				if v := t.Get(alias, "proxyjump"); v != "" {
					return "proxyjump", v
				}
			}
		}
	}

	return "", ""
}

// resolveProxyJump builds the jump transport chain for a comma-separated
// ProxyJump value. Matching OpenSSH, the outermost (first) hop follows its own
// ssh_config proxy directives while later hops' are suppressed by the explicit
// chain.
func (c *Config) resolveProxyJump(value string, seen map[string]bool) (*Host, error) {
	if strings.EqualFold(strings.TrimSpace(value), "none") {
		return nil, nil
	}

	var prev *Host
	first := true

	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		hop, err := c.resolveHost(part, first, seen)
		if err != nil {
			return nil, err
		}

		if !first {
			hop.transport = JumpTransport{Host: prev}
			hop.proxyCommand = ""
			hop.finish()
		}

		first = false
		prev = hop
	}

	return prev, nil
}

func cloneSigners(signers []ssh.Signer) []ssh.Signer {
	if len(signers) == 0 {
		return nil
	}

	cloned := make([]ssh.Signer, 0, len(signers))

	for _, signer := range signers {
		if signer != nil {
			cloned = append(cloned, signer)
		}
	}

	if len(cloned) == 0 {
		return nil
	}

	return cloned
}

func signersEqual(a, b []ssh.Signer) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !signerEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}

func signerEqual(a, b ssh.Signer) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if av.Type() != bv.Type() {
		return false
	}

	if av.Comparable() {
		return av.Interface() == bv.Interface()
	}

	return reflect.DeepEqual(a, b)
}
