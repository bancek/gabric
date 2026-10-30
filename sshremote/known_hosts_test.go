package sshremote_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/bancek/gabric/sshremote"
)

func TestKnownHostsCallback(t *testing.T) {
	srv := startServer(t)

	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(srv.KnownHosts()+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	callback, err := sshremote.KnownHostsCallback(path)
	if err != nil {
		t.Fatalf("KnownHostsCallback() error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: srv.Port()}
	hostname := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Port()))

	if err := callback(hostname, addr, srv.HostKey()); err != nil {
		t.Errorf("KnownHostsCallback() error: %v", err)
	}
}

func TestKnownHostsCallbackMultipleFiles(t *testing.T) {
	srv := startServer(t)

	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	path := filepath.Join(dir, "known_hosts2")
	if err := os.WriteFile(path, []byte(srv.KnownHosts()+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	callback, err := sshremote.KnownHostsCallback(filepath.Join(dir, "missing"), emptyPath, path)
	if err != nil {
		t.Fatalf("KnownHostsCallback() error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: srv.Port()}
	hostname := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Port()))

	if err := callback(hostname, addr, srv.HostKey()); err != nil {
		t.Errorf("KnownHostsCallback() error: %v", err)
	}
}

func TestKnownHostsAcceptNewCallback(t *testing.T) {
	srv := startServer(t)

	path := filepath.Join(t.TempDir(), "known_hosts")
	callback, err := sshremote.KnownHostsAcceptNewCallback(path)
	if err != nil {
		t.Fatalf("KnownHostsAcceptNewCallback() error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: srv.Port()}
	hostname := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Port()))

	if err := callback(hostname, addr, srv.HostKey()); err != nil {
		t.Fatalf("callback() unknown host error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if got, want := string(b), srv.KnownHosts()+"\n"; got != want {
		t.Errorf("known_hosts = %q, want %q", got, want)
	}

	if err := callback(hostname, addr, srv.HostKey()); err != nil {
		t.Errorf("callback() known host error: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	otherKey, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error: %v", err)
	}

	err = callback(hostname, addr, otherKey.PublicKey())
	if err != nil {
		if sshremote.IsUnknownHost(err) {
			t.Errorf("callback() changed key error = %v, want not an unknown host", err)
		}
	} else {
		t.Fatal("callback() changed key = nil, want error")
	}
}

func TestKnownHostsAcceptNewCallbackRequiresFile(t *testing.T) {
	if _, err := sshremote.KnownHostsAcceptNewCallback(); err == nil {
		t.Fatal("KnownHostsAcceptNewCallback() without paths = nil error, want error")
	}

	if _, err := sshremote.KnownHostsAcceptNewCallback(""); err == nil {
		t.Fatal("KnownHostsAcceptNewCallback(\"\") = nil error, want error")
	}
}

func TestKnownHostsAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")

	if err := sshremote.KnownHostsAppend(path, "example.com ssh-rsa AAAA\n"); err != nil {
		t.Fatalf("KnownHostsAppend() error: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("file mode = %o, want 600", got)
		}
	}

	if err := sshremote.KnownHostsAppend(path, "other.com ssh-rsa BBBB\n"); err != nil {
		t.Fatalf("KnownHostsAppend() error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	want := "example.com ssh-rsa AAAA\nother.com ssh-rsa BBBB\n"
	if string(b) != want {
		t.Errorf("known_hosts = %q, want %q", string(b), want)
	}
}

func TestKnownHostsLine(t *testing.T) {
	srv := startServer(t)
	key := srv.HostKey()

	tests := []struct {
		name     string
		hostname string
		want     string
	}{
		{name: "default port", hostname: "example.com", want: "example.com "},
		{name: "host and port", hostname: "example.com:2222", want: "[example.com]:2222 "},
		{name: "ipv6", hostname: "[::1]:2222", want: "[::1]:2222 "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := sshremote.KnownHostsLine(tt.hostname, key)

			if !strings.HasPrefix(line, tt.want) {
				t.Errorf("KnownHostsLine(%q) = %q, want prefix %q", tt.hostname, line, tt.want)
			}

			if !strings.HasSuffix(line, "\n") {
				t.Errorf("KnownHostsLine(%q) = %q, want a trailing newline", tt.hostname, line)
			}
		})
	}
}

func TestIsUnknownHost(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unknown", err: &knownhosts.KeyError{}, want: true},
		{name: "mismatch", err: &knownhosts.KeyError{Want: []knownhosts.KnownKey{{}}}, want: false},
		{name: "revoked", err: &knownhosts.RevokedError{}, want: false},
		{name: "other", err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sshremote.IsUnknownHost(tt.err); got != tt.want {
				t.Errorf("IsUnknownHost(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
