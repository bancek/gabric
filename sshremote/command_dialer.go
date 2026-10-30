package sshremote

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// commandDialer runs a local command and uses its stdio as the transport.
type commandDialer struct {
	host    *Host
	command string
	cmd     *exec.Cmd
}

func (d *commandDialer) Dial(ctx context.Context) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", expandProxyCommand(d.command, d.host))
	cmd.Stderr = os.Stderr

	configureCommand(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()

		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()

		return nil, err
	}

	d.cmd = cmd

	return &pipeConn{reader: stdout, writer: stdin}, nil
}

func (d *commandDialer) Close() error {
	if d.cmd == nil || d.cmd.Process == nil {
		return nil
	}

	_ = killCommand(d.cmd)
	_ = d.cmd.Wait() // reap; the exit status is irrelevant

	return nil
}

// expandProxyCommand expands the ssh ProxyCommand tokens for host: %h (host),
// %p (port), %r (user), %n/%k (host, as no key alias is tracked), and %%.
func expandProxyCommand(command string, host *Host) string {
	var b strings.Builder

	for i := 0; i < len(command); i++ {
		if command[i] != '%' || i+1 == len(command) {
			b.WriteByte(command[i])
			continue
		}

		switch command[i+1] {
		case 'h', 'n', 'k':
			b.WriteString(host.Hostname())
		case 'p':
			b.WriteString(strconv.Itoa(host.Port()))
		case 'r':
			b.WriteString(host.User())
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(command[i+1])
		}

		i++
	}

	return b.String()
}
