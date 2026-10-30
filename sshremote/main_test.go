package sshremote_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric/sshremote"
)

var (
	tcpBridgeAddr = flag.String("gabric-tcp-bridge", "", "internal: dial addr and bridge stdio")
	stdioClient   = flag.Bool("gabric-stdio-client", false, "internal: run the stdio transport test client")
	stdioUser     = flag.String("gabric-stdio-user", "", "internal: stdio client ssh user")
	stdioKey      = flag.String("gabric-stdio-key", "", "internal: stdio client key path")
)

// TestMain lets tests re-execute the test binary as a subprocess helper (see
// tcpBridgeMain and stdioClientMain).
func TestMain(m *testing.M) {
	flag.Parse()

	if *tcpBridgeAddr != "" {
		tcpBridgeMain(*tcpBridgeAddr)
	}

	if *stdioClient {
		stdioClientMain(*stdioUser, *stdioKey)
	}

	os.Exit(m.Run())
}

// stdioClientMain runs a remote command over the process stdin/stdout as the
// SSH transport. The result goes to stderr (stdout is the transport).
func stdioClientMain(user, key string) {
	host := sshremote.NewHost("stdio.invalid").
		WithPort(22).
		WithUser(user).
		WithPrivateKeysPaths([]string{key}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithTransport(sshremote.StdioTransport{})
	config := sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(sshremote.DefaultConnectionTimeout)

	r := sshremote.NewRemote(sshremote.NewHostConfig(host, config))
	defer r.Close()

	var out bytes.Buffer

	err := r.WithSession(context.Background(), testLogger(), func(session *ssh.Session) error {
		session.Stdout = &out
		return session.Run("run")
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stderr, out.String())
	os.Exit(0)
}

// tcpBridgeMain dials addr and copies stdio. It is run as a CommandTransport by
// TestRemoteClientCommandTransport, not as a test.
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
