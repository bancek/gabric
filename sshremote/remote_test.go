package sshremote_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/internal/testutils"
	"github.com/bancek/gabric/sshremote"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func startServer(t *testing.T, opts ...sshtest.Option) *sshtest.Server {
	t.Helper()

	t.Setenv("SSH_AUTH_SOCK", "")

	srv, err := sshtest.Start(opts...)
	if err != nil {
		t.Fatalf("sshtest.Start() error: %v", err)
	}

	t.Cleanup(func() { srv.Close() })

	return srv
}

func testConfig() *sshremote.Config {
	return sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(10 * time.Second)
}

func testContext() context.Context {
	return context.Background()
}

func testHost(srv *sshtest.Server) *sshremote.Host {
	return sshremote.NewHost("127.0.0.1").
		WithPort(srv.Port()).
		WithUser(srv.User()).
		WithPrivateKeysPaths([]string{srv.ClientKeyPath()}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)
}

func testRemote(t *testing.T, srv *sshtest.Server, idleConnections, maxConnections int) *sshremote.Remote {
	t.Helper()

	return testRemoteWithConfig(t, srv, testConfig(), idleConnections, maxConnections)
}

func testRemoteWithConfig(t *testing.T, srv *sshtest.Server, config *sshremote.Config, idleConnections, maxConnections int) *sshremote.Remote {
	t.Helper()

	config = config.WithIdleConnections(idleConnections).WithMaxConnections(maxConnections)
	r := sshremote.NewRemote(sshremote.NewHostConfig(testHost(srv), config))
	t.Cleanup(r.Close)

	return r
}

func testConfigWithSessionTimeout(timeout time.Duration) *sshremote.Config {
	return testConfig().WithSessionOpenTimeout(timeout)
}

func TestRemoteSession(t *testing.T) {
	srv := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "hello")
		return 0
	}))

	r := testRemote(t, srv, 1, 1)

	var out bytes.Buffer

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("echo hello")
	})
	if err != nil {
		t.Fatalf("WithSession() error: %v", err)
	}

	if out.String() != "hello" {
		t.Errorf("output = %q, want %q", out.String(), "hello")
	}
}

func TestRemoteSessionDoesNotProbe(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	for range 2 {
		err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
			return nil
		})
		if err != nil {
			t.Fatalf("WithSession() error: %v", err)
		}
	}

	if got := srv.SessionCount(); got != 2 {
		t.Errorf("sessions = %d, want 2", got)
	}
}

func TestRemoteSessionRefusedKeepsConnection(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	srv.RejectSessions(true)

	err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	})

	if _, ok := errors.AsType[*ssh.OpenChannelError](err); !ok {
		t.Fatalf("WithSession() error = %v, want *ssh.OpenChannelError", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}

	srv.RejectSessions(false)

	if err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	}); err != nil {
		t.Fatalf("WithSession() after refusal error: %v", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("connections = %d, want 1 (a refused session must keep the connection)", got)
	}
}

func TestRemoteSessionDrainsStaleConnections(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 3, 3)

	release := make(chan struct{})
	started := make(chan struct{}, 3)
	errs := make(chan error, 3)

	for range 3 {
		go func() {
			errs <- r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
				started <- struct{}{}
				<-release
				return nil
			})
		}()
	}

	for range 3 {
		<-started
	}

	if got := srv.ConnCount(); got != 3 {
		t.Fatalf("connections = %d, want 3", got)
	}
	if got := srv.CloseConnections(); got != 3 {
		t.Fatalf("closed connections = %d, want 3", got)
	}

	close(release)

	for range 3 {
		if err := <-errs; err != nil {
			t.Errorf("WithSession() error: %v", err)
		}
	}

	if err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	}); err != nil {
		t.Fatalf("WithSession() after stale connections error: %v", err)
	}

	if got := srv.ConnCount(); got != 4 {
		t.Errorf("connections = %d, want 4", got)
	}
}

func TestRemoteSessionOpenTimeout(t *testing.T) {
	srv := startServer(t)

	config := testConfigWithSessionTimeout(50 * time.Millisecond)
	r := testRemoteWithConfig(t, srv, config, 1, 1)

	unblock := srv.BlockSessions()
	defer unblock()

	err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	})
	if !errors.Is(err, sshremote.ErrSessionOpenTimeout) {
		t.Fatalf("WithSession() error = %v, want ErrSessionOpenTimeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WithSession() error = %v, want context.DeadlineExceeded", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}

	unblock()

	if err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	}); err != nil {
		t.Fatalf("WithSession() after timeout error: %v", err)
	}

	if got := srv.ConnCount(); got != 2 {
		t.Errorf("connections = %d, want 2 (a timed-out connection must be discarded)", got)
	}
}

func TestRemoteSessionOpenCanceled(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	unblock := srv.BlockSessions()
	defer unblock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- r.WithSession(ctx, testLogger(), func(*ssh.Session) error {
			return nil
		})
	}()

	testutils.Eventually(t, time.Second, func() bool {
		return srv.SessionCount() == 1
	})

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WithSession() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WithSession() did not return after cancellation")
	}

	if got := srv.ConnCount(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}

	unblock()

	if err := r.WithSession(testContext(), testLogger(), func(*ssh.Session) error {
		return nil
	}); err != nil {
		t.Fatalf("WithSession() after cancellation error: %v", err)
	}

	if got := srv.ConnCount(); got != 2 {
		t.Errorf("connections = %d, want 2 (a canceled open must discard the connection)", got)
	}
}

func TestRemoteSessionTimeoutDoesNotLimitOperation(t *testing.T) {
	srv := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, _, _ io.Writer) int {
		time.Sleep(100 * time.Millisecond)
		return 0
	}))

	config := testConfigWithSessionTimeout(20 * time.Millisecond)
	r := testRemoteWithConfig(t, srv, config, 1, 1)

	err := r.WithSession(testContext(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = io.Discard
		session.Stderr = io.Discard

		return session.Run("slow")
	})
	if err != nil {
		t.Fatalf("WithSession() error: %v", err)
	}
}

func TestRemoteSessionAlreadyCanceled(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.WithSession(ctx, testLogger(), func(*ssh.Session) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WithSession() error = %v, want context.Canceled", err)
	}

	if got := srv.ConnCount(); got != 0 {
		t.Errorf("connections = %d, want 0", got)
	}
}

func TestRemoteSessionPanicReleasesConnection(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("WithSession() did not propagate the callback panic")
			}
		}()

		_ = r.WithSession(context.Background(), testLogger(), func(*ssh.Session) error {
			panic("boom")
		})
	}()

	err := r.WithSession(context.Background(), testLogger(), func(*ssh.Session) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithSession() after panic error: %v", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("connections = %d, want 1", got)
	}
}

func TestRemoteConnectionPool(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 2, 2)

	ctx := testContext()

	release := make(chan struct{})
	holding := make(chan struct{}, 2)
	holderErrs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			holderErrs <- r.WithSession(ctx, testLogger(), func(*ssh.Session) error {
				holding <- struct{}{}
				<-release
				return nil
			})
		}()
	}

	<-holding
	<-holding

	acquired := make(chan struct{})
	thirdErr := make(chan error, 1)

	go func() {
		thirdErr <- r.WithSession(ctx, testLogger(), func(*ssh.Session) error {
			close(acquired)
			return nil
		})
	}()

	testutils.Consistently(t, 200*time.Millisecond, func() bool {
		select {
		case <-acquired:
			t.Fatal("WithSession() acquired a connection beyond MaxConnections")
		case err := <-thirdErr:
			t.Fatalf("WithSession() error: %v", err)
		default:
			return true
		}

		return false
	})

	close(release)

	select {
	case <-acquired:
	case err := <-thirdErr:
		t.Fatalf("WithSession() error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("WithSession() did not proceed after a connection was released")
	}

	for i := 0; i < 2; i++ {
		if err := <-holderErrs; err != nil {
			t.Errorf("WithSession() error: %v", err)
		}
	}

	if got := srv.ConnCount(); got != 2 {
		t.Errorf("connections = %d, want 2", got)
	}
}

func TestRemoteConcurrentSessions(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 2, 2)

	ctx := testContext()

	const workers = 20

	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			errs <- r.WithSession(ctx, testLogger(), func(session *ssh.Session) error {
				session.Stdout = io.Discard
				session.Stderr = io.Discard
				return session.Run("run")
			})
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("WithSession() error: %v", err)
		}
	}

	if got := srv.ConnCount(); got < 1 || got > 2 {
		t.Errorf("connections = %d, want 1..2", got)
	}
}

func TestRemoteCancelSessionReusesConnection(t *testing.T) {
	started := make(chan struct{})
	var startedOnce sync.Once

	srv := startServer(t, sshtest.WithExecHandler(func(_ string, stdin io.Reader, _, _ io.Writer) int {
		startedOnce.Do(func() { close(started) })
		io.Copy(io.Discard, stdin)
		return 0
	}))

	r := testRemote(t, srv, 1, 1)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- r.WithSession(ctx, testLogger(), func(session *ssh.Session) error {
			if _, err := session.StdinPipe(); err != nil {
				return err
			}
			session.Stdout = io.Discard
			session.Stderr = io.Discard
			return session.Run("run")
		})
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WithSession() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WithSession() did not return after cancellation")
	}

	if got := srv.ConnCount(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}

	// Cancellation closed only the session, so the next operation reuses the
	// same connection.
	err := r.WithSession(context.Background(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = io.Discard
		session.Stderr = io.Discard
		return session.Run("run")
	})
	if err != nil {
		t.Fatalf("WithSession() after cancel error: %v", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("connections = %d after reuse, want 1", got)
	}
}

func TestRemoteAcquireCancelled(t *testing.T) {
	srv := startServer(t)

	r := testRemote(t, srv, 1, 1)

	release := make(chan struct{})
	holding := make(chan struct{})

	go func() {
		_ = r.WithSession(context.Background(), testLogger(), func(*ssh.Session) error {
			close(holding)
			<-release
			return nil
		})
	}()

	<-holding

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- r.WithSession(ctx, testLogger(), func(*ssh.Session) error {
			return nil
		})
	}()

	testutils.Consistently(t, 100*time.Millisecond, func() bool {
		select {
		case err := <-done:
			t.Fatalf("WithSession() returned before cancellation: %v", err)
		default:
			return true
		}

		return false
	})

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WithSession() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WithSession() did not return after cancellation")
	}

	close(release)
}

func TestRemoteSessionErrorKeepsConnection(t *testing.T) {
	var calls atomic.Int32

	srv := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, _, _ io.Writer) int {
		if calls.Add(1) == 1 {
			return 1
		}
		return 0
	}))

	r := testRemote(t, srv, 1, 1)

	run := func() error {
		return r.WithSession(context.Background(), testLogger(), func(session *ssh.Session) error {
			session.Stdout = io.Discard
			session.Stderr = io.Discard
			return session.Run("run")
		})
	}

	if err := run(); err == nil {
		t.Fatal("WithSession() = nil, want the remote exit error")
	}

	if err := run(); err != nil {
		t.Fatalf("WithSession() after error error: %v", err)
	}

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("connections = %d, want 1 (a failed command must not discard the connection)", got)
	}
}

func TestRemoteConfigIdentity(t *testing.T) {
	srv := startServer(t)

	host := testHost(srv)
	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)

	ctx := testContext()
	logger := testLogger()
	config := testConfig()

	run := func(config *sshremote.Config) {
		t.Helper()

		r := manager.GetRemote(sshremote.NewHostConfig(host, config))

		err := r.WithSession(ctx, logger, func(*ssh.Session) error {
			return nil
		})
		if err != nil {
			t.Fatalf("WithSession() error: %v", err)
		}
	}

	run(config)

	if got := srv.ConnCount(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}

	// The same config pointer reuses the pooled connection.
	run(config)

	if got := srv.ConnCount(); got != 1 {
		t.Errorf("connections = %d after reuse, want 1", got)
	}

	// An equal but distinct config has a distinct identity.
	run(config.Clone())

	if got := srv.ConnCount(); got != 2 {
		t.Errorf("connections = %d after equal config clone, want 2", got)
	}

	// Changing config produces a new identity.
	changed := config.WithConnectionTimeout(5 * time.Second)

	run(changed)

	if got := srv.ConnCount(); got != 3 {
		t.Errorf("connections = %d after config change, want 3", got)
	}

	// Setting a host key callback produces a new identity.
	withCallback := config.WithHostKeyCallback(func(sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		return ssh.InsecureIgnoreHostKey(), nil
	})

	run(withCallback)

	if got := srv.ConnCount(); got != 4 {
		t.Errorf("connections = %d after setting a callback, want 4", got)
	}

	// The same callback config pointer reuses the connection.
	run(withCallback)

	if got := srv.ConnCount(); got != 4 {
		t.Errorf("connections = %d after reusing the callback config, want 4", got)
	}

	// A clone is a distinct identity.
	run(withCallback.Clone())

	if got := srv.ConnCount(); got != 5 {
		t.Errorf("connections = %d after cloning the callback config, want 5", got)
	}

	// Clearing the callback produces another identity.
	run(withCallback.WithHostKeyCallback(nil))

	if got := srv.ConnCount(); got != 6 {
		t.Errorf("connections = %d after clearing the callback, want 6", got)
	}
}
