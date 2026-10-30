package gabricssh

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/sshremote"
)

func testContext(t *testing.T) context.Context {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return logger.NewLoggerContext(context.Background(), log)
}

func startSSHServer(t *testing.T, opts ...sshtest.Option) *sshtest.Server {
	t.Helper()

	t.Setenv("SSH_AUTH_SOCK", "")

	srv, err := sshtest.Start(opts...)
	if err != nil {
		t.Fatalf("sshtest.Start() error: %v", err)
	}

	t.Cleanup(func() { srv.Close() })

	return srv
}

func remoteServerContext(t *testing.T, srv *sshtest.Server, sudo bool) context.Context {
	t.Helper()

	ctx := testContext(t)

	config := sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(sshremote.DefaultConnectionTimeout).
		WithPrivateKeysPaths([]string{srv.ClientKeyPath()})
	ctx = NewConfigContext(ctx, config)

	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)
	ctx = NewManagerContext(ctx, manager)

	host := sshremote.NewHost("127.0.0.1").
		WithPort(srv.Port()).
		WithUser(srv.User()).
		WithPrivateKeysPaths([]string{srv.ClientKeyPath()}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)
	ctx = NewHostContext(ctx, host)
	ctx = NewRunnerContext(ctx)

	if sudo {
		ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})
	}

	return ctx
}
