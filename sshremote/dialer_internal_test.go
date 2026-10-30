package sshremote

import (
	"context"
	"errors"
	"testing"
	"time"
)

type unsupportedTransport struct{}

func (unsupportedTransport) transport()                     {}
func (unsupportedTransport) ProxyCommand(HostConfig) string { return "" }

func TestNewDialer(t *testing.T) {
	config := NewConfig().WithConnectionTimeout(time.Second)
	host := &Host{hostname: "example.com", port: 2222}

	d, err := newDialer(context.Background(), NewHostConfig(host, config), testInternalLogger())
	if err != nil {
		t.Fatalf("newDialer() error: %v", err)
	}

	direct, ok := d.(directDialer)
	if !ok {
		t.Fatalf("newDialer() = %T, want directDialer", d)
	}
	if direct.addr != "example.com:2222" {
		t.Errorf("addr = %q, want %q", direct.addr, "example.com:2222")
	}
	if direct.timeout != time.Second {
		t.Errorf("timeout = %s, want %s", direct.timeout, time.Second)
	}

	d, err = newDialer(context.Background(), NewHostConfig(&Host{hostname: "example.com", transport: StdioTransport{}}, config), testInternalLogger())
	if err != nil {
		t.Fatalf("newDialer() error: %v", err)
	}
	if _, ok := d.(*stdioDialer); !ok {
		t.Fatalf("newDialer() = %T, want *stdioDialer", d)
	}

	_, err = newDialer(context.Background(), NewHostConfig(&Host{hostname: "example.com", transport: unsupportedTransport{}}, config), testInternalLogger())
	if err != nil {
		return
	}

	t.Fatal("newDialer() with unsupported transport = nil, want error")
}

func TestDirectDialerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := (directDialer{addr: "127.0.0.1:1", timeout: time.Second}).Dial(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Dial() error = %v, want context.Canceled", err)
	}
}
