package gabricssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/internal/testutils"
)

func TestForwardEcho(t *testing.T) {
	echoAddr := startEcho(t)

	srv := startSSHServer(t, sshtest.WithDirectTCPIP())
	ctx := remoteServerContext(t, srv, false)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fwd, errCh := startForward(t, ctx, func(f *ForwardOps) *ForwardOps {
		return f.Local("127.0.0.1:0").Remote(echoAddr)
	})

	forwardRoundTrip(t, fwd.LocalAddr())

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("ConnCount() = %d, want 1 (dedicated connection)", got)
	}

	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunErr() = %v, want context.Canceled", err)
	}
}

func TestForwardCancellation(t *testing.T) {
	echoAddr := startEcho(t)

	srv := startSSHServer(t, sshtest.WithDirectTCPIP())
	ctx := remoteServerContext(t, srv, false)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fwd, errCh := startForward(t, ctx, func(f *ForwardOps) *ForwardOps {
		return f.Local("127.0.0.1:0").Remote(echoAddr)
	})

	addr := fwd.LocalAddr()

	cancel()

	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunErr() = %v, want context.Canceled", err)
	}

	testutils.Eventually(t, 5*time.Second, func() bool {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return true
		}
		conn.Close()

		return false
	})
}

func TestForwardRefusesNonLoopback(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithDirectTCPIP())
	ctx := remoteServerContext(t, srv, false)

	err := Forward(ctx).
		Local("0.0.0.0:0").
		Remote("127.0.0.1:1").
		RunErr()
	if err != nil {
		return
	}

	t.Fatal("RunErr() on a non-loopback bind = nil, want error")
}

func TestForwardAllowRemote(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithDirectTCPIP())
	ctx := remoteServerContext(t, srv, false)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, errCh := startForward(t, ctx, func(f *ForwardOps) *ForwardOps {
		return f.AllowRemote().Local("0.0.0.0:0").Remote("127.0.0.1:1")
	})

	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunErr() = %v, want context.Canceled", err)
	}
}

func TestForwardNoHost(t *testing.T) {
	err := Forward(context.Background()).Remote("127.0.0.1:1").RunErr()
	if err != nil {
		return
	}

	t.Fatal("RunErr() without host = nil, want error")
}

func TestForwardRemoteUnset(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithDirectTCPIP())
	ctx := remoteServerContext(t, srv, false)

	if err := Forward(ctx).RunErr(); err == nil {
		t.Fatal("RunErr() without remote address = nil, want error")
	}
}

func TestParseForwardArg(t *testing.T) {
	tests := []struct {
		arg        string
		wantLocal  string
		wantRemote string
	}{
		{arg: "", wantLocal: "", wantRemote: ""},
		{arg: "127.0.0.1:8080", wantLocal: "127.0.0.1:8080", wantRemote: "127.0.0.1:8080"},
		{arg: "127.0.0.1:9090,127.0.0.1:8080", wantLocal: "127.0.0.1:9090", wantRemote: "127.0.0.1:8080"},
		{arg: ",127.0.0.1:8080", wantLocal: "", wantRemote: "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		local, remote := parseForwardArg(tt.arg)
		if local != tt.wantLocal || remote != tt.wantRemote {
			t.Errorf("parseForwardArg(%q) = %q, %q, want %q, %q", tt.arg, local, remote, tt.wantLocal, tt.wantRemote)
		}
	}
}

func TestRegisterForwardTask(t *testing.T) {
	g := gabric.New()

	RegisterForwardTask(g)

	if _, err := g.ParseTasks([]string{"forward"}); err != nil {
		t.Fatalf("ParseTasks(forward) error: %v", err)
	}
}

func TestRegisterForwardTaskDuplicate(t *testing.T) {
	g := gabric.New()

	RegisterForwardTask(g)

	defer func() {
		if recover() == nil {
			t.Fatal("RegisterForwardTask() twice did not panic")
		}
	}()

	RegisterForwardTask(g)
}

// startForward starts f().RunErr in a goroutine and returns the ops (from which
// LocalAddr can be read) and the error channel. It waits until the local
// listener is bound.
func startForward(t *testing.T, ctx context.Context, build func(*ForwardOps) *ForwardOps) (*ForwardOps, <-chan error) {
	t.Helper()

	opsCh := make(chan *ForwardOps, 1)
	errCh := make(chan error, 1)

	go func() {
		f := build(Forward(ctx))
		opsCh <- f
		errCh <- f.RunErr()
	}()

	fwd := <-opsCh

	testutils.Eventually(t, 5*time.Second, func() bool {
		return fwd.LocalAddr() != ""
	})

	return fwd, errCh
}

// startEcho starts a local TCP echo server and returns its address.
func startEcho(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error: %v", err)
	}

	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				io.Copy(conn, conn)
			}()
		}
	}()

	return ln.Addr().String()
}

// forwardRoundTrip dials addr, writes a message and asserts it is echoed.
func forwardRoundTrip(t *testing.T, addr string) {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("net.Dial(%q) error: %v", addr, err)
	}
	defer conn.Close()

	msg := []byte("hello forward")

	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	buf := make([]byte, len(msg))

	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull() error: %v", err)
	}

	if !bytes.Equal(buf, msg) {
		t.Errorf("echo = %q, want %q", buf, msg)
	}
}
