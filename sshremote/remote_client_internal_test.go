package sshremote

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

type fakeConn struct {
	closeErr error
}

func (c *fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *fakeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *fakeConn) Close() error                     { return c.closeErr }
func (c *fakeConn) LocalAddr() net.Addr              { return nil }
func (c *fakeConn) RemoteAddr() net.Addr             { return nil }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestRemoteClientClose(t *testing.T) {
	if err := (&RemoteClient{}).Close(); err != nil {
		t.Errorf("Close() with nothing to close = %v, want nil", err)
	}

	sentinel := errors.New("conn close")

	err := (&RemoteClient{conn: &fakeConn{closeErr: sentinel}}).Close()
	if !errors.Is(err, sentinel) {
		t.Errorf("Close() error = %v, want %v", err, sentinel)
	}

	if !strings.Contains(err.Error(), "conn close") {
		t.Errorf("Close() error = %v, want it to contain %q", err, "conn close")
	}
}

func testInternalLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRemoteClientCancelledBeforeDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := NewConfig().WithStrictHostKeyChecking(StrictHostKeyCheckingNo).WithConnectionTimeout(time.Second)
	host := &Host{hostname: "127.0.0.1", port: 1, strictHostKeyChecking: StrictHostKeyCheckingNo}

	if _, err := NewRemoteClient(ctx, NewHostConfig(host, config), testInternalLogger()); !errors.Is(err, context.Canceled) {
		t.Fatalf("NewRemoteClient() error = %v, want context.Canceled", err)
	}
}

func TestNewRemoteClientCancelledHandshake(t *testing.T) {
	// A listener that never speaks SSH: the client blocks in the handshake
	// until the context deadline fires.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer listener.Close()

	config := NewConfig().WithStrictHostKeyChecking(StrictHostKeyCheckingNo).WithConnectionTimeout(time.Second)
	host := &Host{hostname: "127.0.0.1", port: listener.Addr().(*net.TCPAddr).Port, strictHostKeyChecking: StrictHostKeyCheckingNo}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := NewRemoteClient(ctx, NewHostConfig(host, config), testInternalLogger()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewRemoteClient() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestNewRemoteClientCancelledProxyHandshake(t *testing.T) {
	// The jump host stalls in the handshake, so cancellation propagates out of
	// the proxy recursion.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer listener.Close()

	proxy := &Host{hostname: "127.0.0.1", port: listener.Addr().(*net.TCPAddr).Port, strictHostKeyChecking: StrictHostKeyCheckingNo}
	host := &Host{hostname: "target.invalid", port: 22, strictHostKeyChecking: StrictHostKeyCheckingNo, transport: JumpTransport{Host: proxy}}

	config := NewConfig().WithStrictHostKeyChecking(StrictHostKeyCheckingNo).WithConnectionTimeout(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := NewRemoteClient(ctx, NewHostConfig(host, config), testInternalLogger()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewRemoteClient() error = %v, want context.DeadlineExceeded", err)
	}
}
