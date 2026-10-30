package sshremote_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/bancek/gabric/sshremote"
)

func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error: %v", err)
	}

	return signer
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey() error: %v", err)
	}

	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	return path
}

func TestGetSSHSignersPrivateKey(t *testing.T) {
	path := writeTestPrivateKey(t)

	config := sshremote.NewConfig()

	signers, err := sshremote.GetSSHSigners(sshremote.NewHostConfig(
		sshremote.NewHost("example.com").WithPrivateKeysPaths([]string{path}),
		config,
	))
	if err != nil {
		t.Fatalf("GetSSHSigners() error: %v", err)
	}

	if len(signers) != 1 {
		t.Fatalf("len(signers) = %d, want 1", len(signers))
	}

	if _, err := signers[0].Sign(rand.Reader, []byte("data")); err != nil {
		t.Errorf("Sign() error: %v", err)
	}
}

func TestGetSSHSignersInMemory(t *testing.T) {
	signer := newTestSigner(t)

	config := sshremote.NewConfig().WithSigners(signer)

	signers, err := sshremote.GetSSHSigners(sshremote.NewHostConfig(
		sshremote.NewHost("example.com"),
		config,
	))
	if err != nil {
		t.Fatalf("GetSSHSigners() error: %v", err)
	}

	if len(signers) != 1 {
		t.Fatalf("len(signers) = %d, want 1", len(signers))
	}

	if _, err := signers[0].Sign(rand.Reader, []byte("data")); err != nil {
		t.Errorf("Sign() error: %v", err)
	}
}

func TestGetSSHSignersDeadAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	path := writeTestPrivateKey(t)

	config := sshremote.NewConfig().
		WithAgentSocket(filepath.Join(t.TempDir(), "missing.sock"))

	signers, err := sshremote.GetSSHSigners(sshremote.NewHostConfig(
		sshremote.NewHost("example.com").WithPrivateKeysPaths([]string{path}),
		config,
	))
	if !errors.Is(err, sshremote.ErrAgentUnavailable) {
		t.Fatalf("GetSSHSigners() error = %v, want ErrAgentUnavailable", err)
	}

	if signers != nil {
		t.Errorf("len(signers) = %d, want 0", len(signers))
	}
}

func TestGetSSHSignersAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	keyring := agent.NewKeyring()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	if err := keyring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatalf("keyring.Add() error: %v", err)
	}

	// Short temp dir to stay under the unix socket path length limit.
	dir, err := os.MkdirTemp("", "gabric-agent")
	if err != nil {
		t.Fatalf("MkdirTemp() error: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "agent.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go agent.ServeAgent(keyring, conn)
		}
	}()

	config := sshremote.NewConfig().WithAgentSocket(socketPath)

	signers, err := sshremote.GetSSHSigners(sshremote.NewHostConfig(sshremote.NewHost("example.com"), config))
	if err != nil {
		t.Fatalf("GetSSHSigners() error: %v", err)
	}

	if len(signers) != 1 {
		t.Fatalf("len(signers) = %d, want 1", len(signers))
	}

	// The listing connection is already closed, so Sign must re-dial.
	if _, err := signers[0].Sign(rand.Reader, []byte("data")); err != nil {
		t.Errorf("Sign() with agent running error: %v", err)
	}

	listener.Close()

	if _, err := signers[0].Sign(rand.Reader, []byte("data")); err == nil {
		t.Error("Sign() with agent stopped = nil, want error")
	}
}
