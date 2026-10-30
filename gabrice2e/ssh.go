package gabrice2e

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricdocker"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/internal/testutils"
	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/sshremote"
)

// SSHTest is a Gabric target backed by a real sshd in a Docker container.
type SSHTest struct {
	*Test

	container *Container
	host      *sshremote.Host
	config    *sshremote.Config
	manager   *sshremote.RemoteManager
	signer    ssh.Signer
	user      string
	baseCtx   context.Context
}

// SSH starts an SSH server container for user and returns an SSH target. The
// user must exist in the image with a passwd and primary group entry.
func SSH(t testing.TB, image Image, user string, opts ...ContainerOption) *SSHTest {
	t.Helper()

	if user == "" {
		t.Fatal("gabrice2e: empty SSH user")
	}

	opts = append(append([]ContainerOption(nil), opts...), WithContainerPort(22))

	c := RunContainer(t, image, opts...)

	baseCtx := logger.NewLoggerContext(
		t.Context(),
		slog.New(logger.NewTextHandler(io.Discard, nil)),
	)

	dockerOps := gabricdocker.Remote(baseCtx, c.Name())

	home, group, err := resolveSSHUser(dockerOps, user)
	if err != nil {
		t.Fatalf("gabrice2e: resolve SSH user %q: %v", user, err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gabrice2e: generate SSH key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("gabrice2e: create SSH signer: %v", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("gabrice2e: create SSH public key: %v", err)
	}

	sshDir := path.Join(home, ".ssh")

	if err := dockerOps.Mkdir(sshDir).
		Recursive().
		Mode(0o700).
		User(user).
		Group(group).
		RunErr(); err != nil {
		t.Fatalf("gabrice2e: create authorized_keys directory: %v", err)
	}

	if err := dockerOps.Put(path.Join(sshDir, "authorized_keys")).
		Bytes(ssh.MarshalAuthorizedKey(sshPublicKey)).
		Mode(0o600).
		User(user).
		Group(group).
		RunErr(); err != nil {
		t.Fatalf("gabrice2e: install authorized_keys: %v", err)
	}

	config := sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(sshremote.DefaultConnectionTimeout).
		WithSigners(signer)

	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)

	host := sshremote.NewHost("127.0.0.1").
		WithPort(c.Port(22)).
		WithUser(user).
		WithSigners(signer).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

	ctx := sshContext(baseCtx, config, manager, host)

	testutils.Eventually(t, 30*time.Second, func() bool {
		return gabric.Remote(ctx).Exec("true").RunErr() == nil
	})

	return &SSHTest{
		Test: &Test{
			t:   t,
			ctx: ctx,
		},
		container: c,
		host:      host,
		config:    config,
		manager:   manager,
		signer:    signer,
		user:      user,
		baseCtx:   baseCtx,
	}
}

// Container returns the target container.
func (g *SSHTest) Container() *Container {
	return g.container
}

// Host returns the host used for direct test connections.
func (g *SSHTest) Host() *sshremote.Host {
	return g.host
}

// User returns the SSH login user.
func (g *SSHTest) User() string {
	return g.user
}

// NetworkHost returns the host name and port for connections made from another
// container on the same Docker network.
func (g *SSHTest) NetworkHost() *sshremote.Host {
	return sshremote.NewHost(g.container.Name()).
		WithPort(22).
		WithUser(g.user).
		WithSigners(g.signer).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)
}

// WithHost returns a new SSHTest that uses host for its target context.
func (g *SSHTest) WithHost(host *sshremote.Host) *SSHTest {
	return &SSHTest{
		Test: &Test{
			t:   g.T(),
			ctx: sshContext(g.baseCtx, g.config, g.manager, host),
		},
		container: g.container,
		host:      host,
		config:    g.config,
		manager:   g.manager,
		signer:    g.signer,
		user:      g.user,
		baseCtx:   g.baseCtx,
	}
}

// WithHostTransport returns a new SSHTest with transport installed on the host.
func (g *SSHTest) WithHostTransport(transport sshremote.Transport) *SSHTest {
	return g.WithHost(g.Host().WithTransport(transport))
}

func resolveSSHUser(ops *gabric.RemoteOps, user string) (home, group string, err error) {
	passwd, err := ops.Get("/etc/passwd").StringErr()
	if err != nil {
		return "", "", fmt.Errorf("read /etc/passwd: %w", err)
	}

	var gid string

	for line := range strings.SplitSeq(passwd, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 7 || fields[0] != user {
			continue
		}

		home = fields[5]
		gid = fields[3]

		break
	}

	if home == "" {
		return "", "", fmt.Errorf("user %q has no passwd entry", user)
	}

	groups, err := ops.Get("/etc/group").StringErr()
	if err != nil {
		return "", "", fmt.Errorf("read /etc/group: %w", err)
	}

	for line := range strings.SplitSeq(groups, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 || fields[2] != gid {
			continue
		}

		group = fields[0]

		break
	}

	if group == "" {
		return "", "", fmt.Errorf("user %q has no primary group for gid %q", user, gid)
	}

	return home, group, nil
}

func sshContext(ctx context.Context, config *sshremote.Config, manager *sshremote.RemoteManager, host *sshremote.Host) context.Context {
	ctx = gabricssh.NewConfigContext(ctx, config)
	ctx = gabricssh.NewManagerContext(ctx, manager)
	ctx = gabricssh.NewHostContext(ctx, host)
	ctx = gabricssh.NewRunnerContext(ctx)
	ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)

	return ctx
}
