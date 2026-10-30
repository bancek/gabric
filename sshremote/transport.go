package sshremote

import "net"

// Transport describes how a host's SSH byte stream is obtained. It is a sealed
// union; implementations live in this package. A nil Transport means a direct
// TCP connection to the host's own address.
type Transport interface {
	transport()
	// ProxyCommand renders the transport as a system-ssh ProxyCommand (the
	// value passed to `-o ProxyCommand=`). It is used for the system-ssh
	// fallback when Host.ProxyCommand() is empty.
	ProxyCommand(hc HostConfig) string
}

// JumpTransport connects through an SSH jump host and forwards to this host's
// own address.
type JumpTransport struct {
	Host *Host
}

func (JumpTransport) transport() {}

func (t JumpTransport) ProxyCommand(hc HostConfig) string {
	if t.Host == nil {
		return ""
	}

	return hc.WithHost(t.Host).SSHCommand() + " nc %h %p"
}

// CommandTransport runs a local command and uses its stdio as the transport (a
// ProxyCommand that cannot be mapped to another transport).
type CommandTransport struct {
	Command string
}

func (CommandTransport) transport() {}

func (t CommandTransport) ProxyCommand(HostConfig) string {
	return t.Command
}

// DialTransport dials Addr (host:port) directly instead of the host's own
// address (a ProxyCommand "nc host port").
type DialTransport struct {
	Addr string
}

func (DialTransport) transport() {}

func (t DialTransport) ProxyCommand(HostConfig) string {
	host, port, err := net.SplitHostPort(t.Addr)
	if err != nil {
		return "nc " + t.Addr
	}

	return "nc " + host + " " + port
}

// ForwardTransport connects through Host and forwards to Addr (a different
// address), e.g. an "ssh dest nc other 22" ProxyCommand.
type ForwardTransport struct {
	Host *Host
	Addr string
}

func (ForwardTransport) transport() {}

func (t ForwardTransport) ProxyCommand(hc HostConfig) string {
	if t.Host == nil {
		return ""
	}

	host, port, err := net.SplitHostPort(t.Addr)
	if err != nil {
		return hc.WithHost(t.Host).SSHCommand() + " nc " + t.Addr
	}

	return hc.WithHost(t.Host).SSHCommand() + " nc " + host + " " + port
}

// SessionCommandTransport runs Command in a shell session on Host and bridges
// its stdio as the transport, e.g. an "ssh dest <cmd>" ProxyCommand.
type SessionCommandTransport struct {
	Host    *Host
	Command string
}

func (SessionCommandTransport) transport() {}

func (t SessionCommandTransport) ProxyCommand(hc HostConfig) string {
	if t.Host == nil {
		return ""
	}

	return hc.WithHost(t.Host).SSHCommand() + " " + t.Command
}

// StdioTransport uses the process stdin/stdout as the transport
// (ProxyCommand "-").
type StdioTransport struct{}

func (StdioTransport) transport() {}

func (StdioTransport) ProxyCommand(HostConfig) string {
	return "-"
}
