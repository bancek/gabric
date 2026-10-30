package sshremote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"time"
)

// dialer opens the byte stream a RemoteClient's SSH connection runs over, and
// owns whatever must stay alive for that stream (for example a nested jump
// client).
type dialer interface {
	Dial(ctx context.Context) (net.Conn, error)
	Close() error
}

// directDialer dials addr over TCP.
type directDialer struct {
	addr    string
	timeout time.Duration
}

func (d directDialer) Dial(ctx context.Context) (net.Conn, error) {
	return (&net.Dialer{Timeout: d.timeout}).DialContext(ctx, "tcp", d.addr)
}

func (directDialer) Close() error { return nil }

// jumpDialer forwards to addr through the jump client.
type jumpDialer struct {
	jump *RemoteClient
	addr string
}

func (d *jumpDialer) Dial(ctx context.Context) (net.Conn, error) {
	return d.jump.Client().DialContext(ctx, "tcp", d.addr)
}

func (d *jumpDialer) Close() error {
	if d.jump == nil {
		return nil
	}

	return d.jump.Close()
}

// newJumpDialer connects through hc and forwards to addr.
func newJumpDialer(ctx context.Context, hc HostConfig, addr string, logger *slog.Logger) (dialer, error) {
	jump, err := NewRemoteClient(ctx, hc, logger)
	if err != nil {
		return nil, err
	}

	return &jumpDialer{jump: jump, addr: addr}, nil
}

// pipeConn adapts a pair of streams (a command's stdio, or a session's pipes)
// to net.Conn for the SSH transport. Deadlines are no-ops; connection
// establishment uses Close to unblock a stuck handshake.
type pipeConn struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (c *pipeConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *pipeConn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

func (c *pipeConn) Close() error {
	return errors.Join(c.reader.Close(), c.writer.Close())
}

func (c *pipeConn) LocalAddr() net.Addr {
	return pipeAddr{}
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// newDialer builds the dialer for hc. It is the one place each Transport case
// maps to a dialer.
func newDialer(ctx context.Context, hc HostConfig, logger *slog.Logger) (dialer, error) {
	host := hc.Host()

	switch transport := host.Transport().(type) {
	case nil:
		return directDialer{addr: host.GetHost(), timeout: hc.Config().ConnectionTimeout()}, nil
	case JumpTransport:
		return newJumpDialer(ctx, hc.WithHost(transport.Host), host.GetHost(), logger)
	case ForwardTransport:
		return newJumpDialer(ctx, hc.WithHost(transport.Host), transport.Addr, logger)
	case SessionCommandTransport:
		jump, err := NewRemoteClient(ctx, hc.WithHost(transport.Host), logger)
		if err != nil {
			return nil, err
		}

		return &sessionDialer{host: host, jump: jump, command: transport.Command}, nil
	case StdioTransport:
		return &stdioDialer{stdin: os.Stdin, stdout: os.Stdout}, nil
	case DialTransport:
		return directDialer{addr: transport.Addr, timeout: hc.Config().ConnectionTimeout()}, nil
	case CommandTransport:
		return &commandDialer{host: host, command: transport.Command}, nil
	default:
		return nil, fmt.Errorf("unsupported transport: %T", transport)
	}
}
