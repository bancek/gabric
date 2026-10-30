package gabricssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/sshremote"
)

// ForwardOps builds a local port forward (ssh -L) over the remote host from
// context. The zero value is not usable; create one with Forward.
type ForwardOps struct {
	ctx         context.Context
	localAddr   string
	remoteAddr  string
	allowRemote bool

	mu        sync.Mutex
	boundAddr string
}

// Forward returns a builder for a local port forward over the remote host from
// the context. The local side defaults to a loopback ephemeral port. The
// forward owns a dedicated connection.
func Forward(ctx context.Context) *ForwardOps {
	return &ForwardOps{ctx: ctx, localAddr: "127.0.0.1:0"}
}

// Local sets the local bind address. It defaults to "127.0.0.1:0". A
// non-loopback bind is refused unless AllowRemote is called.
func (f *ForwardOps) Local(addr string) *ForwardOps {
	f.localAddr = addr
	return f
}

// Remote sets the target "host:port" as reached from the SSH server (for
// example "127.0.0.1:8080").
func (f *ForwardOps) Remote(addr string) *ForwardOps {
	f.remoteAddr = addr
	return f
}

// AllowRemote permits binding the local listener to a non-loopback address. It
// is off by default because a forward is an easy way to expose an internal
// service by accident.
func (f *ForwardOps) AllowRemote() *ForwardOps {
	f.allowRemote = true
	return f
}

// LocalAddr returns the bound local address, or an empty string before RunErr
// has started listening. It is useful when the configured port is 0.
func (f *ForwardOps) LocalAddr() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.boundAddr
}

func (f *ForwardOps) setLocalAddr(addr string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.boundAddr = addr
}

// Run blocks, forwarding connections, until ctx is cancelled. It panics on
// failure.
func (f *ForwardOps) Run() *ForwardOps {
	if err := f.RunErr(); err != nil {
		panic(err)
	}
	return f
}

// RunErr forwards connections until ctx is cancelled. It returns ctx.Err() on
// cancellation; per-connection errors are logged, not fatal.
func (f *ForwardOps) RunErr() error {
	if !HasHost(f.ctx) {
		return errors.New("no host: pass -ssh-host")
	}

	if f.remoteAddr == "" {
		return errors.New("forward remote address not set")
	}

	if !f.allowRemote {
		if err := checkLoopback(f.localAddr); err != nil {
			return err
		}
	}

	ln, err := net.Listen("tcp", f.localAddr)
	if err != nil {
		return err
	}
	defer ln.Close()

	f.setLocalAddr(ln.Addr().String())

	logger := gabric.Logger(f.ctx)

	run := func(client *ssh.Client) error {
		logger.With("local", ln.Addr().String(), "remote", f.remoteAddr).Info("Local forward listening")

		return f.serve(client, ln)
	}

	return sshremote.WithNewRemoteClient(f.ctx, GetHostConfig(f.ctx), logger, run)
}

// serve accepts local connections and forwards them over client until ctx is
// cancelled or the listener fails.
func (f *ForwardOps) serve(client *ssh.Client, ln net.Listener) error {
	ctx := f.ctx

	stop := context.AfterFunc(ctx, func() { ln.Close() })
	defer stop()

	var wg sync.WaitGroup

	defer wg.Wait()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		wg.Go(func() {
			f.handle(client, conn)
		})
	}
}

// handle opens a remote channel to the target address and copies bytes in both
// directions until either side closes.
func (f *ForwardOps) handle(client *ssh.Client, local net.Conn) {
	defer local.Close()

	remoteConn, err := client.DialContext(f.ctx, "tcp", f.remoteAddr)
	if err != nil {
		gabric.Logger(f.ctx).With("error", err).Debug("Local forward dial error")

		return
	}
	defer remoteConn.Close()

	gabric.Logger(f.ctx).With("local", local.RemoteAddr().String(), "remote", f.remoteAddr).Debug("Local forward connection")

	done := make(chan struct{}, 2)

	go func() {
		io.Copy(remoteConn, local)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(local, remoteConn)
		done <- struct{}{}
	}()

	<-done
}

// checkLoopback returns an error when addr does not bind a loopback interface.
func checkLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	if host == "localhost" {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}

	return fmt.Errorf("refusing to bind non-loopback address %q: call AllowRemote() to override", addr)
}
