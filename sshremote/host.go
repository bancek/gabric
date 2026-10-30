package sshremote

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/user"
	"slices"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
)

var currentUser = sync.OnceValue(func() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
})

// Host describes an immutable SSH host.
type Host struct {
	displayName           string
	hostname              string
	port                  int
	user                  string
	proxyCommand          string
	transport             Transport
	privateKeysPaths      []string
	sshSigners            []ssh.Signer
	knownHostsPaths       []string
	strictHostKeyChecking StrictHostKeyChecking

	identityValue hostIdentity
}

// NewHost returns a Host with the given hostname.
func NewHost(hostname string) *Host {
	h := &Host{hostname: hostname}

	return h.finish()
}

// DisplayName returns the optional human-readable name used in logs.
func (h *Host) DisplayName() string {
	return h.displayName
}

// Hostname returns the hostname or IP address to connect to.
func (h *Host) Hostname() string {
	return h.hostname
}

// Port returns the effective SSH port, defaulting to 22.
func (h *Host) Port() int {
	if h.port == 0 {
		return 22
	}
	return h.port
}

// User returns the effective SSH user, falling back to the current OS user.
func (h *Host) User() string {
	if h.user != "" {
		return h.user
	}
	return currentUser()
}

// ProxyCommand returns the original ssh_config ProxyCommand.
func (h *Host) ProxyCommand() string {
	return h.proxyCommand
}

// Transport returns how to reach this host. Nil means a direct connection.
func (h *Host) Transport() Transport {
	return h.transport
}

// PrivateKeysPaths returns the host-specific private key files. The effective
// list, including config defaults, is available from HostConfig.
func (h *Host) PrivateKeysPaths() []string {
	return slices.Clone(h.privateKeysPaths)
}

// Signers returns the host-specific in-memory signers.
func (h *Host) Signers() []ssh.Signer {
	return cloneSigners(h.sshSigners)
}

// KnownHostsPaths returns the host-specific known_hosts files. The effective
// list, including config defaults, is available from HostConfig.
func (h *Host) KnownHostsPaths() []string {
	return slices.Clone(h.knownHostsPaths)
}

// StrictHostKeyChecking returns the host-specific host key policy. Empty means
// the config default applies; use HostConfig for the effective policy.
func (h *Host) StrictHostKeyChecking() StrictHostKeyChecking {
	return h.strictHostKeyChecking
}

// GetHost returns the host in host:port form.
func (h *Host) GetHost() string {
	return net.JoinHostPort(h.hostname, strconv.Itoa(h.Port()))
}

// SSHHost returns the [user@]host destination, without a port. IPv6 addresses
// are wrapped in square brackets as required by rsync. Advanced: this renders
// a destination for the system ssh binary.
func (h *Host) SSHHost() string {
	host := bracketIPv6(h.hostname)

	if user := h.User(); user != "" {
		host = user + "@" + host
	}

	return host
}

// String returns a human readable representation of the host.
func (h *Host) String() string {
	s := h.GetHost()
	if u := h.User(); u != "" {
		s = u + "@" + s
	}
	if h.displayName == "" {
		return s
	}
	return h.displayName + " (" + s + ")"
}

// WithDisplayName returns a copy with the display name replaced.
func (h *Host) WithDisplayName(name string) *Host {
	clone := h.clone()
	clone.displayName = name
	return clone.finish()
}

// WithHostname returns a copy with the hostname replaced.
func (h *Host) WithHostname(hostname string) *Host {
	clone := h.clone()
	clone.hostname = hostname
	return clone.finish()
}

// WithPort returns a copy with the port replaced.
func (h *Host) WithPort(port int) *Host {
	clone := h.clone()
	clone.port = port
	return clone.finish()
}

// WithUser returns a copy with the user replaced.
func (h *Host) WithUser(user string) *Host {
	clone := h.clone()
	clone.user = user
	return clone.finish()
}

// WithProxyCommand returns a copy with the raw ProxyCommand replaced.
func (h *Host) WithProxyCommand(command string) *Host {
	clone := h.clone()
	clone.proxyCommand = command
	return clone.finish()
}

// WithTransport returns a copy with the transport replaced.
func (h *Host) WithTransport(transport Transport) *Host {
	clone := h.clone()
	clone.transport = transport
	return clone.finish()
}

// WithPrivateKeysPaths returns a copy with the private key paths replaced.
func (h *Host) WithPrivateKeysPaths(paths []string) *Host {
	clone := h.clone()
	clone.privateKeysPaths = slices.Clone(paths)
	return clone.finish()
}

// WithSigners returns a copy with the in-memory signers replaced.
func (h *Host) WithSigners(signers ...ssh.Signer) *Host {
	clone := h.clone()
	clone.sshSigners = cloneSigners(signers)
	return clone.finish()
}

// WithKnownHostsPaths returns a copy with the known_hosts paths replaced.
func (h *Host) WithKnownHostsPaths(paths []string) *Host {
	clone := h.clone()
	clone.knownHostsPaths = slices.Clone(paths)
	return clone.finish()
}

// WithStrictHostKeyChecking returns a copy with the host key policy replaced.
func (h *Host) WithStrictHostKeyChecking(policy StrictHostKeyChecking) *Host {
	clone := h.clone()
	clone.strictHostKeyChecking = policy
	return clone.finish()
}

// Clone returns an independent copy of h.
func (h *Host) Clone() *Host {
	return h.clone().finish()
}

func (h *Host) clone() *Host {
	clone := *h
	clone.privateKeysPaths = slices.Clone(h.privateKeysPaths)
	clone.sshSigners = cloneSigners(h.sshSigners)
	clone.knownHostsPaths = slices.Clone(h.knownHostsPaths)
	return &clone
}

// ParseHost parses a host string such as "user@127.0.0.1:2222".
func ParseHost(host string) (*Host, error) {
	// e.g. user@127.0.0.1:2222
	u, err := url.Parse("ssh://" + host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh host: %s: %w", host, err)
	}

	h := &Host{hostname: u.Hostname()}

	if portStr := u.Port(); portStr != "" {
		port, _ := strconv.Atoi(portStr)
		h.port = port
	}

	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			return nil, errors.New("passwords are not supported")
		}
		h.user = u.User.Username()
	}

	return h.finish(), nil
}

type hostIdentitySimple struct {
	hostname string
	port     int
	user     string
}

// hostIdentity identifies a Host for RemoteManager caching. Simple hosts use
// their basic connection identity; hosts with other connection fields use
// pointer identity.
type hostIdentity struct {
	simple hostIdentitySimple
	host   *Host
}

func (h *Host) identity() hostIdentity {
	return h.identityValue
}

func (h *Host) computeIdentity() hostIdentity {
	if h.isSimple() {
		return hostIdentity{
			simple: hostIdentitySimple{
				hostname: h.hostname,
				port:     h.Port(),
				user:     h.User(),
			},
		}
	}

	return hostIdentity{host: h}
}

func (h *Host) finish() *Host {
	h.identityValue = h.computeIdentity()
	return h
}

func (h *Host) isSimple() bool {
	return h.displayName == "" &&
		h.proxyCommand == "" &&
		h.transport == nil &&
		len(h.privateKeysPaths) == 0 &&
		len(h.sshSigners) == 0 &&
		len(h.knownHostsPaths) == 0 &&
		h.strictHostKeyChecking == ""
}
