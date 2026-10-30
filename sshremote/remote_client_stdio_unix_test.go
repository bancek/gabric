//go:build unix

package sshremote_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/bancek/gabric/internal/sshtest"
)

func TestRemoteClientStdioTransport(t *testing.T) {
	target := startServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "target")
		return 0
	}))

	tcpConn, err := net.Dial("tcp", target.Addr())
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}

	tcp, ok := tcpConn.(*net.TCPConn)
	if !ok {
		tcpConn.Close()
		t.Fatalf("connection is %T, want *net.TCPConn", tcpConn)
	}

	file, err := tcp.File()
	tcpConn.Close()
	if err != nil {
		t.Fatalf("File() error: %v", err)
	}
	t.Cleanup(func() { file.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, os.Args[0],
		"-gabric-stdio-client",
		"-gabric-stdio-user", target.User(),
		"-gabric-stdio-key", target.ClientKeyPath(),
	)
	cmd.Stdin = file
	cmd.Stdout = file
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("stdio client error: %v\n%s", err, stderr.String())
	}

	if got := stderr.String(); got != "target" {
		t.Errorf("stdio client output = %q, want %q", got, "target")
	}
}
