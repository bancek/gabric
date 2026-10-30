package sshremote

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestStdioDialer(t *testing.T) {
	var out bytes.Buffer

	d := &stdioDialer{stdin: strings.NewReader("hello"), stdout: &out}

	conn, err := d.Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}

	buf := make([]byte, len("hello"))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("Read() = %q, want %q", buf, "hello")
	}

	if _, err := conn.Write([]byte("world")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if out.String() != "world" {
		t.Errorf("Write() wrote %q, want %q", out.String(), "world")
	}

	if err := conn.Close(); err != nil {
		t.Errorf("conn.Close() error: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestStdioDialerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := &stdioDialer{stdin: strings.NewReader(""), stdout: io.Discard}

	if _, err := d.Dial(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Dial() error = %v, want context.Canceled", err)
	}
}
