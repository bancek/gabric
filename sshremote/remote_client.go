package sshremote

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// RemoteClient is a single SSH client connection whose byte stream is provided
// by a dialer (a direct dial, a jump host, a command, a session, or stdio).
// Advanced: use WithNewRemoteClient when a dedicated connection is needed.
type RemoteClient struct {
	dialer dialer
	conn   net.Conn
	client *ssh.Client
}

// NewRemoteClient dials and authenticates an SSH connection for hc, logging to
// logger. The context bounds connection establishment (the TCP dial and the SSH
// handshake); once connected, the context no longer affects the connection.
// Advanced: use WithNewRemoteClient when the connection only needs a scoped
// callback.
func NewRemoteClient(ctx context.Context, hc HostConfig, logger *slog.Logger) (rc *RemoteClient, err error) {
	host := hc.Host()

	logger = logger.With("host", host.String())

	sshAuth, err := HostSSHAuth(hc, logger)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := hc.HostKeyCallback()
	if err != nil {
		return nil, err
	}

	d, err := newDialer(ctx, hc, logger)
	if err != nil {
		return nil, err
	}

	remoteClient := &RemoteClient{dialer: d}
	defer func() {
		if err != nil {
			remoteClient.Close()
		}
	}()

	hostStr := host.GetHost()

	logger.Debug("RemoteClient client dial")

	var conn net.Conn

	conn, err = d.Dial(ctx)
	if err != nil {
		logger.With("error", err).Debug("RemoteClient client dial error")
		return nil, err
	}

	remoteClient.conn = conn

	sshConfig := &ssh.ClientConfig{
		User:            host.User(),
		Auth:            []ssh.AuthMethod{sshAuth},
		HostKeyCallback: hostKeyCallback,
		Timeout:         hc.Config().ConnectionTimeout(),
	}

	logger.With("config", sshConfig).Debug("RemoteClient new client conn")

	stop := context.AfterFunc(ctx, func() { conn.Close() })

	c, chans, reqs, err := ssh.NewClientConn(conn, hostStr, sshConfig)
	if !stop() {
		// ctx is done and the conn was closed to unblock the handshake.
		logger.Debug("RemoteClient new client conn cancelled")

		return nil, ctx.Err()
	}
	if err != nil {
		logger.With("error", err).Debug("RemoteClient new client conn error")
		return nil, err
	}

	logger.Debug("RemoteClient new client")

	remoteClient.client = ssh.NewClient(c, chans, reqs)

	return remoteClient, nil
}

// Close closes the client and any proxy connection.
func (c *RemoteClient) Close() error {
	var errs []error

	if c.client != nil {
		errs = append(errs, c.client.Close())
	}
	if c.conn != nil {
		errs = append(errs, c.conn.Close())
	}
	if c.dialer != nil {
		errs = append(errs, c.dialer.Close())
	}

	return errors.Join(errs...)
}

// Client returns the underlying ssh client.
func (c *RemoteClient) Client() *ssh.Client {
	return c.client
}

// WithNewRemoteClient establishes a dedicated SSH connection for hc and runs f
// with it, closing the connection when it returns. Cancelling ctx closes the
// connection (unblocking anything using it) and makes WithNewRemoteClient
// return ctx.Err(). Advanced: this bypasses the pooled Remote.
func WithNewRemoteClient(ctx context.Context, hc HostConfig, logger *slog.Logger, f func(client *ssh.Client) error) error {
	client, err := NewRemoteClient(ctx, hc, logger)
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeClient := func() { closeOnce.Do(func() { client.Close() }) }

	stop := context.AfterFunc(ctx, closeClient)
	defer stop()
	defer closeClient()

	err = f(client.Client())
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}
