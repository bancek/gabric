package gabricsshe2e

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/kballard/go-shellquote"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrice2e"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/internal/e2eimages"
	"github.com/bancek/gabric/internal/testutils"
	"github.com/bancek/gabric/sshremote"
)

const testSSHUser = "gabric"

var tcpBridgeAddr = flag.String("gabric-tcp-bridge", "", "internal: dial addr and bridge stdio")

func TestMain(m *testing.M) {
	flag.Parse()

	if *tcpBridgeAddr != "" {
		tcpBridgeMain(*tcpBridgeAddr)
	}

	gabrice2e.Main(m)
}

func newSSH(t *testing.T) *gabrice2e.SSHTest {
	t.Helper()

	return gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser)
}

func TestShell(t *testing.T) {
	g := newSSH(t)

	var out bytes.Buffer

	err := gabricssh.Shell(g.Context()).
		Stdin(strings.NewReader("echo marker; exit\n")).
		Stdout(&out).
		Stderr(io.Discard).
		RunErr()
	if err != nil {
		t.Fatalf("Shell() error: %v", err)
	}

	if !strings.Contains(out.String(), "marker") {
		t.Errorf("shell output = %q, want it to contain %q", out.String(), "marker")
	}
}

func TestShellPTY(t *testing.T) {
	g := newSSH(t)

	var out bytes.Buffer

	err := gabricssh.Shell(g.Context()).
		Stdin(strings.NewReader("echo ptymarker; exit\n")).
		Stdout(&out).
		Stderr(io.Discard).
		Pty(true).
		Term("xterm").
		RunErr()
	if err != nil {
		t.Fatalf("Shell() error: %v", err)
	}

	if !strings.Contains(out.String(), "ptymarker") {
		t.Errorf("shell output = %q, want it to contain %q", out.String(), "ptymarker")
	}
}

func TestSudoUser(t *testing.T) {
	g := newSSH(t)
	ctx := gabric.NewRemoteSudoContext(g.Context(), &gabric.Sudo{
		User:  "app",
		Flags: []string{"-n"},
	})

	out := gabric.Remote(ctx).Exec("id", "-un").CaptureStdout().Run().StdoutStr()
	if got := strings.TrimSpace(out); got != "app" {
		t.Fatalf("sudo -u app id -un = %q, want app", got)
	}

	path := "/home/app/e2e-sudo-user.txt"

	if err := gabric.Remote(ctx).Put(path).String("app data").RunErr(); err != nil {
		t.Fatalf("Put() as app error: %v", err)
	}

	if got := strings.TrimSpace(gabric.Remote(ctx).Exec("cat", path).CaptureStdout().Run().StdoutStr()); got != "app data" {
		t.Errorf("cat as app = %q, want %q", got, "app data")
	}

	stat, err := gabric.Remote(ctx).Stat(path).RunErr()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if stat.User != "app" || stat.Group != "app" {
		t.Errorf("owner = %q:%q, want app:app", stat.User, stat.Group)
	}
}

func TestSSHUser(t *testing.T) {
	g := gabrice2e.SSH(t, e2eimages.DockerSSH, "app")

	if got, want := g.User(), "app"; got != want {
		t.Fatalf("User() = %q, want %q", got, want)
	}

	out := gabric.Remote(g.Context()).Exec("id", "-un").CaptureStdout().Run().StdoutStr()
	if got, want := strings.TrimSpace(out), "app"; got != want {
		t.Fatalf("id -un = %q, want %q", got, want)
	}
}

func TestSSHExitError(t *testing.T) {
	g := newSSH(t)

	err := gabric.Remote(g.Context()).Exec("sh", "-c", "exit 3").RunErr()

	exitErr, ok := errors.AsType[*ssh.ExitError](err)
	if !ok {
		t.Fatalf("error = %T, want *ssh.ExitError", err)
	}
	if exitErr.ExitStatus() != 3 {
		t.Errorf("ExitStatus() = %d, want 3", exitErr.ExitStatus())
	}
}

func TestProxyJump(t *testing.T) {
	network := gabrice2e.RunNetwork(t)

	jump := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)
	target := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)

	host := target.NetworkHost().
		WithTransport(sshremote.JumpTransport{
			Host: jump.Host(),
		})

	g := target.WithHost(host)

	out := gabric.Remote(g.Context()).Exec("echo", "jumped").CaptureStdout().Run().StdoutStr()
	if got, want := out, "jumped\n"; got != want {
		t.Errorf("Exec() through proxy = %q, want %q", got, want)
	}
}

func TestCommandTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ProxyCommand exec is unix-only")
	}

	g := newSSH(t)

	host := g.Host().
		WithTransport(sshremote.CommandTransport{
			Command: shellquote.Join(os.Args[0], "-gabric-tcp-bridge", "%h:%p"),
		})

	cmd := g.WithHost(host)

	out := gabric.Remote(cmd.Context()).Exec("echo", "cmd").CaptureStdout().Run().StdoutStr()
	if got, want := out, "cmd\n"; got != want {
		t.Errorf("Exec() through command transport = %q, want %q", got, want)
	}
}

func TestSessionTransport(t *testing.T) {
	network := gabrice2e.RunNetwork(t)

	jump := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)
	target := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)

	host := target.NetworkHost().
		WithTransport(sshremote.SessionCommandTransport{
			Host:    jump.Host(),
			Command: "nc " + target.Container().Name() + " 22",
		})

	g := target.WithHost(host)

	out := gabric.Remote(g.Context()).Exec("echo", "session").CaptureStdout().Run().StdoutStr()
	if got, want := out, "session\n"; got != want {
		t.Errorf("Exec() through session transport = %q, want %q", got, want)
	}
}

func TestForwardTransport(t *testing.T) {
	network := gabrice2e.RunNetwork(t)

	jump := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)
	target := gabrice2e.SSH(t, e2eimages.DockerSSH, testSSHUser,
		gabrice2e.WithContainerNetwork(network),
	)

	host := target.NetworkHost().
		WithTransport(sshremote.ForwardTransport{
			Host: jump.Host(),
			Addr: net.JoinHostPort(target.Container().Name(), "22"),
		})

	g := target.WithHost(host)

	out := gabric.Remote(g.Context()).Exec("echo", "forward").CaptureStdout().Run().StdoutStr()
	if got, want := out, "forward\n"; got != want {
		t.Errorf("Exec() through forward transport = %q, want %q", got, want)
	}
}

func TestLocalForward(t *testing.T) {
	g := newSSH(t)

	ctx, cancel := context.WithCancel(g.Context())
	defer cancel()

	opsCh := make(chan *gabricssh.ForwardOps, 1)
	errCh := make(chan error, 1)

	go func() {
		f := gabricssh.Forward(ctx).
			Local("127.0.0.1:0").
			Remote("127.0.0.1:22")

		opsCh <- f
		errCh <- f.RunErr()
	}()

	fwd := <-opsCh

	testutils.Eventually(t, 10*time.Second, func() bool {
		return fwd.LocalAddr() != ""
	})

	forwarded := g.Host().
		WithHostname("forwarded.invalid").
		WithPort(22).
		WithTransport(sshremote.DialTransport{Addr: fwd.LocalAddr()})

	out := gabric.Remote(g.WithHost(forwarded).Context()).
		Exec("echo", "forwarded").
		CaptureStdout().
		Run().
		StdoutStr()
	if got, want := out, "forwarded\n"; got != want {
		t.Errorf("Exec() through local forward = %q, want %q", got, want)
	}

	cancel()

	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunErr() = %v, want context.Canceled", err)
	}
}

func TestDialTransport(t *testing.T) {
	g := newSSH(t)

	host := g.Host().
		WithHostname("dial-transport.invalid").
		WithPort(22).
		WithTransport(sshremote.DialTransport{Addr: g.Container().Addr(22)})

	dial := g.WithHost(host)

	out := gabric.Remote(dial.Context()).Exec("echo", "dial").CaptureStdout().Run().StdoutStr()
	if got, want := out, "dial\n"; got != want {
		t.Errorf("Exec() through dial transport = %q, want %q", got, want)
	}
}

func tcpBridgeMain(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		os.Exit(2)
	}

	go func() {
		io.Copy(conn, os.Stdin)
		conn.Close()
	}()

	io.Copy(os.Stdout, conn)

	os.Exit(0)
}
