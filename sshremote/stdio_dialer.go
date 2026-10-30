package sshremote

import (
	"context"
	"io"
	"net"
)

// stdioDialer uses the process stdin/stdout as the transport.
type stdioDialer struct {
	stdin  io.Reader
	stdout io.Writer
}

func (d *stdioDialer) Dial(ctx context.Context) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &pipeConn{
		reader: nopReadCloser{d.stdin},
		writer: nopWriteCloser{d.stdout},
	}, nil
}

// Close never closes the process stdio.
func (d *stdioDialer) Close() error { return nil }

type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
