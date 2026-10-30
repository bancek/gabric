package sshremote

import (
	"net"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"
)

// HostConfig pairs a host with the connection config used to reach it.
type HostConfig struct {
	host   *Host
	config *Config
}

// hostConfigIdentity identifies a HostConfig for RemoteManager caching.
type hostConfigIdentity struct {
	host   hostIdentity
	config *Config
}

// NewHostConfig returns the resolver for host and config.
func NewHostConfig(host *Host, config *Config) HostConfig {
	if host == nil {
		panic("nil host")
	}
	if config == nil {
		panic("nil config")
	}

	return HostConfig{host: host, config: config}
}

// Host returns the host.
func (hc HostConfig) Host() *Host {
	return hc.host
}

// Config returns the connection config.
func (hc HostConfig) Config() *Config {
	return hc.config
}

func (hc HostConfig) identity() hostConfigIdentity {
	return hostConfigIdentity{host: hc.host.identity(), config: hc.config}
}

// WithHost returns a copy with the host replaced.
func (hc HostConfig) WithHost(host *Host) HostConfig {
	return NewHostConfig(host, hc.config)
}

// WithConfig returns a copy with the config replaced.
func (hc HostConfig) WithConfig(config *Config) HostConfig {
	return NewHostConfig(hc.host, config)
}

// PrivateKeysPaths returns the host private keys followed by the config keys.
func (hc HostConfig) PrivateKeysPaths() []string {
	keys := hc.host.PrivateKeysPaths()
	return append(keys, hc.config.PrivateKeysPaths()...)
}

// Signers returns the host in-memory signers followed by the config signers.
func (hc HostConfig) Signers() []ssh.Signer {
	signers := hc.host.Signers()
	return append(signers, hc.config.Signers()...)
}

// KnownHostsPaths returns the host known_hosts override, or the config
// defaults when the host has none.
func (hc HostConfig) KnownHostsPaths() []string {
	if paths := hc.host.KnownHostsPaths(); len(paths) > 0 {
		return paths
	}
	return hc.config.KnownHostsPaths()
}

// StrictHostKeyChecking returns the host override, or the config default when
// the host has none.
func (hc HostConfig) StrictHostKeyChecking() StrictHostKeyChecking {
	if policy := hc.host.StrictHostKeyChecking(); policy != "" {
		return policy
	}
	return hc.config.StrictHostKeyChecking()
}

// HostKeyCallback returns the callback configured on the config, or a callback
// built from the effective strict policy and known_hosts paths.
func (hc HostConfig) HostKeyCallback() (ssh.HostKeyCallback, error) {
	if hc.config.hostKeyCallback != nil {
		return hc.config.hostKeyCallback.fn(hc)
	}

	policy := hc.StrictHostKeyChecking()

	switch {
	case policy.acceptNew():
		return KnownHostsAcceptNewCallback(hc.KnownHostsPaths()...)
	case policy.verifyKnownHosts():
		return KnownHostsCallback(hc.KnownHostsPaths()...)
	default:
		return ssh.InsecureIgnoreHostKey(), nil
	}
}

// bracketIPv6 wraps an IPv6 literal host in square brackets, reusing
// net.JoinHostPort's rule and stripping the empty port it appends.
func bracketIPv6(host string) string {
	return strings.TrimSuffix(net.JoinHostPort(host, ""), ":")
}

// SSHHost returns the [user@]host destination, without a port. IPv6 addresses
// are wrapped in square brackets as required by rsync. Advanced: this renders
// arguments for the system ssh binary.
func (hc HostConfig) SSHHost() string {
	return hc.host.SSHHost()
}

// SSHArgs returns ssh options for spawning ssh to this host, e.g. for rsync's
// --rsh. The ssh binary and the destination are not included. Advanced: this
// renders arguments for the system ssh binary.
func (hc HostConfig) SSHArgs() []string {
	args := []string{}

	for _, key := range hc.PrivateKeysPaths() {
		args = append(args, "-i", key)
	}

	if hc.host.port != 0 {
		args = append(args, "-p", strconv.Itoa(hc.host.Port()))
	}

	if proxyCommand := hc.ProxyCommand(); proxyCommand != "" {
		args = append(args, "-o", "ProxyCommand="+proxyCommand)
	}

	if knownHostsPaths := hc.KnownHostsPaths(); len(knownHostsPaths) > 0 {
		args = append(args, "-o", "UserKnownHostsFile="+strings.Join(knownHostsPaths, " "))
	}

	return args
}

// SSHCommand returns a full ssh command for this host, including the
// destination and SSHArgs options. Advanced: this renders a command for the
// system ssh binary.
func (hc HostConfig) SSHCommand() string {
	cmd := append([]string{"ssh", hc.SSHHost()}, hc.SSHArgs()...)

	return shellquote.Join(cmd...)
}

// ProxyCommand returns the original ProxyCommand when set, otherwise one
// derived from the transport. Advanced: this renders the value for the system
// ssh binary.
func (hc HostConfig) ProxyCommand() string {
	if command := hc.host.ProxyCommand(); command != "" {
		return command
	}
	if hc.host.Transport() == nil {
		return ""
	}
	return hc.host.Transport().ProxyCommand(hc)
}

// String returns a human readable representation of the host.
func (hc HostConfig) String() string {
	return hc.host.String()
}
