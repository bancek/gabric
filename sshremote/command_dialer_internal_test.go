//go:build unix

package sshremote

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestExpandProxyCommand(t *testing.T) {
	host := &Host{hostname: "target.example.com", port: 2222, user: "deploy"}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "no tokens", in: "nc proxy 22", want: "nc proxy 22"},
		{name: "host and port", in: "nc %h %p", want: "nc target.example.com 2222"},
		{name: "user", in: "ssh %r@%h -p %p", want: "ssh deploy@target.example.com -p 2222"},
		{name: "percent", in: "echo 100%%", want: "echo 100%"},
		{name: "n and k", in: "%n %k", want: "target.example.com target.example.com"},
		{name: "unknown", in: "%x %", want: "%x %"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandProxyCommand(tt.in, host); got != tt.want {
				t.Errorf("expandProxyCommand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	if got := expandProxyCommand("%p", &Host{hostname: "h"}); got != "22" {
		t.Errorf("default port = %q, want %q", got, "22")
	}
}

func TestCommandDialerDialCancelled(t *testing.T) {
	d := &commandDialer{host: &Host{hostname: "127.0.0.1", port: 22}, command: "sleep 60"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := d.Dial(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Dial() error = %v, want context.Canceled", err)
	}
}

func TestCommandDialerCloseReaps(t *testing.T) {
	d := &commandDialer{host: &Host{hostname: "127.0.0.1", port: 22}, command: "sleep 60"}

	conn, err := d.Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	defer conn.Close()

	if d.cmd == nil || d.cmd.Process == nil {
		t.Fatal("Dial() did not start a process")
	}

	proc := d.cmd.Process

	if err := d.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if err := proc.Signal(os.Kill); !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("Signal() after Close = %v, want os.ErrProcessDone", err)
	}
}

func TestNewRemoteClientCommandHandshakeCancelled(t *testing.T) {
	host := &Host{
		hostname:              "127.0.0.1",
		port:                  22,
		strictHostKeyChecking: StrictHostKeyCheckingNo,
		transport:             CommandTransport{Command: "sleep 60"},
	}
	config := NewConfig().WithStrictHostKeyChecking(StrictHostKeyCheckingNo).WithConnectionTimeout(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if _, err := NewRemoteClient(ctx, NewHostConfig(host, config), testInternalLogger()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewRemoteClient() error = %v, want context.DeadlineExceeded", err)
	}
}
