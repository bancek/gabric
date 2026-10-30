package sshremote

import (
	"context"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// sessionDialer runs a command in a session on the jump host and bridges its
// stdio as the transport.
type sessionDialer struct {
	host    *Host // target, for %h/%p expansion
	jump    *RemoteClient
	command string

	session *ssh.Session
	in      *io.PipeReader
	out     *io.PipeWriter
}

func (d *sessionDialer) Dial(ctx context.Context) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	session, err := d.jump.Client().NewSession()
	if err != nil {
		return nil, err
	}

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	session.Stdin = inR
	session.Stdout = outW
	session.Stderr = os.Stderr

	if err := session.Start(expandProxyCommand(d.command, d.host)); err != nil {
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		session.Close()

		return nil, err
	}

	d.session = session
	d.in = inR
	d.out = outW

	return &pipeConn{reader: outR, writer: inW}, nil
}

func (d *sessionDialer) Close() error {
	if d.session != nil {
		d.session.Close()
	}
	if d.in != nil {
		d.in.Close()
	}
	if d.out != nil {
		d.out.Close()
	}
	if d.session != nil {
		_ = d.session.Wait() // reap; the exit status is irrelevant
	}

	return nil
}
