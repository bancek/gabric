package sshremote_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/sshremote"
)

func TestRemoteClientAuth(t *testing.T) {
	srv := startServer(t)

	client, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(testHost(srv), testConfig()), testLogger())
	if err != nil {
		t.Fatalf("NewRemoteClient() error: %v", err)
	}

	client.Close()
}

func TestRemoteClientDeadAgentFallsBackToKey(t *testing.T) {
	srv := startServer(t)

	config := testConfig().
		WithAgentSocket(filepath.Join(t.TempDir(), "missing.sock"))

	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, nil))

	client, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(testHost(srv), config), logger)
	if err != nil {
		t.Fatalf("NewRemoteClient() with dead agent error: %v", err)
	}
	client.Close()

	if !strings.Contains(buf.String(), "ssh agent unavailable") {
		t.Errorf("log = %q, want ssh agent fallback warning", buf.String())
	}
}

func TestRemoteClientUnauthorized(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error: %v", err)
	}

	srv := startServer(t, sshtest.WithAuthorizedKey(signer.PublicKey()))

	if _, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(testHost(srv), testConfig()), testLogger()); err == nil {
		t.Fatal("NewRemoteClient() with unauthorized key = nil, want error")
	}
}

func TestRemoteClientKnownHosts(t *testing.T) {
	srv := startServer(t)

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(srv.KnownHosts()+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	host := testHost(srv)
	host = host.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes)

	config := testConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingYes).
		WithKnownHostsPaths([]string{
			filepath.Join(t.TempDir(), "missing_known_hosts"),
			knownHostsPath,
		})

	client, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger())
	if err != nil {
		t.Fatalf("NewRemoteClient() with known host error: %v", err)
	}
	client.Close()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	otherSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Port()))
	config = config.WithKnownHostsPaths([]string{filepath.Join(t.TempDir(), "known_hosts")})

	if err := os.WriteFile(config.KnownHostsPaths()[0], []byte(knownhosts.Line([]string{addr}, otherSigner.PublicKey())+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if _, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger()); err == nil {
		t.Fatal("NewRemoteClient() with wrong host key = nil, want error")
	}
}

func TestRemoteClientStrictHostKeyCheckingAcceptNew(t *testing.T) {
	srv := startServer(t)

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")

	config := testConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew).
		WithKnownHostsPaths([]string{knownHostsPath})

	host := testHost(srv)
	host = host.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew)

	client, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger())
	if err != nil {
		t.Fatalf("NewRemoteClient() with accept-new error: %v", err)
	}
	client.Close()

	b, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if got, want := string(b), srv.KnownHosts()+"\n"; got != want {
		t.Errorf("known_hosts = %q, want %q", got, want)
	}

	client, err = sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger())
	if err != nil {
		t.Fatalf("NewRemoteClient() with a known host under accept-new error: %v", err)
	}
	client.Close()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	otherSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.Port()))
	if err := os.WriteFile(knownHostsPath, []byte(knownhosts.Line([]string{addr}, otherSigner.PublicKey())+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if _, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger()); err == nil {
		t.Fatal("NewRemoteClient() with a changed key under accept-new = nil, want error")
	}
}

func TestRemoteClientHostKeyCallback(t *testing.T) {
	srv := startServer(t)

	called := 0

	config := testConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
			called++
			return ssh.InsecureIgnoreHostKey(), nil
		})

	host := testHost(srv)
	host = host.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

	client, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger())
	if err != nil {
		t.Fatalf("NewRemoteClient() error: %v", err)
	}
	client.Close()

	if called == 0 {
		t.Error("HostKeyCallback was not used")
	}
}

func TestRemoteClientHostKeyCallbackError(t *testing.T) {
	srv := startServer(t)

	sentinel := errors.New("callback failed")

	config := testConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
			return nil, sentinel
		})

	host := testHost(srv)
	host = host.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

	if _, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger()); !errors.Is(err, sentinel) {
		t.Fatalf("NewRemoteClient() error = %v, want %v", err, sentinel)
	}
}

func TestRemoteClientProxyJump(t *testing.T) {
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))
	proxy := startServer(t, sshtest.WithDirectTCPIP())

	host := testHost(target).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithTransport(sshremote.JumpTransport{
			Host: testHost(proxy).WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo),
		})

	config := testConfig()

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, config))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through proxy error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q (reached target through proxy)", out.String(), "target")
	}
}

const proxyJumpFromSSHConfigTemplate = `
Host jump
    HostName 127.0.0.1
    Port %d
    User %s
    IdentityFile %s
    StrictHostKeyChecking no

Host target
    HostName 127.0.0.1
    Port %d
    User %s
    IdentityFile %s
    StrictHostKeyChecking no
    ProxyJump jump
`

func TestRemoteClientProxyJumpFromSSHConfig(t *testing.T) {
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))
	proxy := startServer(t, sshtest.WithDirectTCPIP())

	sshConfig := fmt.Sprintf(proxyJumpFromSSHConfigTemplate,
		proxy.Port(), proxy.User(), proxy.ClientKeyPath(),
		target.Port(), target.User(), target.ClientKeyPath())

	sshConfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	config := testConfig().WithSSHConfigPath(sshConfigPath)

	host, err := config.ResolveHost("target")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if jumpHost(t, host.Transport()).Port() != proxy.Port() {
		t.Fatalf("jump host = %+v, want the jump host resolved from ssh_config", host.Transport())
	}

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, config))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err = r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through ssh_config proxy error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q", out.String(), "target")
	}
}

func TestRemoteClientProxyDisabled(t *testing.T) {
	target := startServer(t)
	proxy := startServer(t)

	host := testHost(target).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithTransport(sshremote.JumpTransport{
			Host: testHost(proxy).WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo),
		})

	config := testConfig()

	if _, err := sshremote.NewRemoteClient(testContext(), sshremote.NewHostConfig(host, config), testLogger()); err == nil {
		t.Fatal("NewRemoteClient() with disabled direct-tcpip = nil, want error")
	}
}

func TestRemoteClientDialTransport(t *testing.T) {
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))

	host := testHost(target).
		WithHostname("dial-transport.invalid").
		WithPort(22).
		WithTransport(sshremote.DialTransport{Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(target.Port()))})

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, testConfig()))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through dial transport error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q (dialed the DialTransport address)", out.String(), "target")
	}
}

func TestRemoteClientForwardTransport(t *testing.T) {
	jump := startServer(t, sshtest.WithDirectTCPIP())
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))

	host := testHost(target).
		WithHostname("forward-transport.invalid").
		WithPort(22).
		WithTransport(sshremote.ForwardTransport{
			Host: testHost(jump).WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo),
			Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(target.Port())),
		})

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, testConfig()))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through forward transport error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q (forwarded to the Addr via the jump)", out.String(), "target")
	}
}

func TestRemoteClientSessionTransport(t *testing.T) {
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))

	// The jump session command connector: dial the target and bridge stdio.
	jump := startServer(t, sshtest.WithExecHandler(func(_ string, stdin io.Reader, stdout, _ io.Writer) int {
		conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(target.Port())))
		if err != nil {
			return 1
		}
		defer conn.Close()

		go func() {
			io.Copy(conn, stdin)
			conn.Close()
		}()

		io.Copy(stdout, conn)

		return 0
	}))

	host := testHost(target).
		WithHostname("session-transport.invalid").
		WithPort(22).
		WithTransport(sshremote.SessionCommandTransport{
			Host:    testHost(jump).WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo),
			Command: "connect",
		})

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, testConfig()))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through session transport error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q (reached target through the session command)", out.String(), "target")
	}
}

func TestRemoteClientCommandTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ProxyCommand exec is unix-only")
	}

	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))

	command := shellquote.Join(os.Args[0], "-gabric-tcp-bridge", "%h:%p")

	host := testHost(target).
		WithTransport(sshremote.CommandTransport{Command: command})

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, testConfig()))
	t.Cleanup(r.Close)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() through command transport error: %v", err)
	}

	if out.String() != "target" {
		t.Errorf("output = %q, want %q", out.String(), "target")
	}
}

func TestWithNewRemoteClient(t *testing.T) {
	srv := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "ok")
		return 0
	}))

	var out bytes.Buffer

	err := sshremote.WithNewRemoteClient(testContext(), sshremote.NewHostConfig(testHost(srv), testConfig()), testLogger(), func(client *ssh.Client) error {
		session, err := client.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()

		session.Stdout = &out

		return session.Run("echo ok")
	})
	if err != nil {
		t.Fatalf("WithNewRemoteClient() error: %v", err)
	}

	if out.String() != "ok" {
		t.Errorf("output = %q, want %q", out.String(), "ok")
	}
}

func TestWithNewRemoteClientCancelled(t *testing.T) {
	srv := startServer(t)

	ctx, cancel := context.WithCancel(testContext())

	clientCh := make(chan *ssh.Client, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- sshremote.WithNewRemoteClient(ctx, sshremote.NewHostConfig(testHost(srv), testConfig()), testLogger(), func(client *ssh.Client) error {
			clientCh <- client
			<-ctx.Done()

			return nil
		})
	}()

	client := <-clientCh

	cancel()

	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("WithNewRemoteClient() error = %v, want context.Canceled", err)
	}

	if _, err := client.NewSession(); err == nil {
		t.Error("NewSession() after cancellation = nil, want error (connection closed)")
	}
}

func TestWithNewRemoteClientConnectError(t *testing.T) {
	host := sshremote.NewHost("127.0.0.1").
		WithPort(1).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

	called := false

	err := sshremote.WithNewRemoteClient(testContext(), sshremote.NewHostConfig(host, testConfig()), testLogger(), func(*ssh.Client) error {
		called = true

		return nil
	})
	if err != nil {
		if called {
			t.Error("f was called despite a connection error")
		}

		return
	}

	t.Fatal("WithNewRemoteClient() with an unreachable host = nil, want error")
}
