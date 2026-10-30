package gabric

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
)

// errRsyncTransportUnsupported is returned when the native transport cannot be
// used (non-unix platform, where inherited file descriptors are unavailable).
var errRsyncTransportUnsupported = errors.New("gabric rsync transport unsupported")

// rsyncRshCommand returns the --rsh value: the current executable re-executed
// in sidecar mode. rsync appends the host and the remote server command.
func rsyncRshCommand() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}

	return shellquote.Join(exe, rsyncSidecarArg)
}

// rsyncTransport bridges a local rsync process to a remote rsync server over the
// context's remote runner (installed by the transport), without the system ssh
// binary. The local rsync's remote shell is the gabric binary itself,
// re-executed in sidecar mode (rsyncSidecarArg), which proxies rsync's stdio
// over inherited file descriptors.
type rsyncTransport struct {
	rsh         string
	dataParent  *os.File
	dataChild   *os.File
	argsRead    *os.File
	argsWrite   *os.File
	statusRead  *os.File
	statusWrite *os.File
}

func newRsyncTransport() (*rsyncTransport, error) {
	dataParent, dataChild, err := newSocketPair()
	if err != nil {
		return nil, err
	}

	argsRead, argsWrite, err := os.Pipe()
	if err != nil {
		dataParent.Close()
		dataChild.Close()
		return nil, err
	}

	statusRead, statusWrite, err := os.Pipe()
	if err != nil {
		dataParent.Close()
		dataChild.Close()
		argsRead.Close()
		argsWrite.Close()
		return nil, err
	}

	return &rsyncTransport{
		rsh:         rsyncRshCommand(),
		dataParent:  dataParent,
		dataChild:   dataChild,
		argsRead:    argsRead,
		argsWrite:   argsWrite,
		statusRead:  statusRead,
		statusWrite: statusWrite,
	}, nil
}

// Rsh returns the --rsh value.
func (t *rsyncTransport) Rsh() string {
	return t.rsh
}

// ChildFiles returns the extra files passed to the local rsync process as fds
// 3, 4 and 5.
func (t *rsyncTransport) ChildFiles() []*os.File {
	return []*os.File{t.dataChild, t.argsWrite, t.statusRead}
}

// CloseChildFiles closes the parent's copies of the files handed to the child.
// It runs once the child process has started.
func (t *rsyncTransport) CloseChildFiles() {
	t.dataChild.Close()
	t.argsWrite.Close()
	t.statusRead.Close()
}

// Serve reads the remote command from the rsh helper and runs it through the
// context's remote runner, bridging the rsync protocol between the helper and
// the runner.
func (t *rsyncTransport) Serve(ctx context.Context) error {
	command, err := readRsyncServerCommand(t.argsRead)
	if err != nil {
		t.finish(1)
		return err
	}

	Logger(ctx).With("cmd", command).Debug("Rsync remote server")

	// The socketpair end is handed to the runner directly. A local child's stdio
	// is switched to blocking mode by os/exec, so the remote rsync server does
	// not see EAGAIN; a Go runner keeps using the runtime poller.
	runErr := GetRemoteRunner(ctx)(ctx, command, t.dataParent, t.dataParent, os.Stderr)

	status := 0
	if exitErr, ok := errors.AsType[*ExecExitError](runErr); ok {
		status = exitErr.Status
	} else if runErr != nil {
		status = 1
	}

	t.finish(status)

	return runErr
}

// finish signals end of stream to the helper and reports the remote exit
// status.
func (t *rsyncTransport) finish(status int) {
	t.dataParent.Close()
	fmt.Fprintf(t.statusWrite, "%d\n", status)
	t.statusWrite.Close()
}

// CloseArgs closes the rsh argument pipe, unblocking Serve when the helper was
// never started (for example, when local rsync failed to exec).
func (t *rsyncTransport) CloseArgs() {
	t.argsRead.Close()
	t.argsWrite.Close()
}

// Close closes all transport files. It is safe to call more than once.
func (t *rsyncTransport) Close() {
	t.dataParent.Close()
	t.dataChild.Close()
	t.argsRead.Close()
	t.argsWrite.Close()
	t.statusRead.Close()
	t.statusWrite.Close()
}

// readRsyncServerCommand reads the NUL framed argument list written by the
// sidecar (a count field followed by that many fields) and returns the remote
// server command. It reads exactly the framed message and does not wait for
// EOF: the local rsync process also inherits the write end of the argument
// pipe, so EOF does not arrive until after the transfer.
func readRsyncServerCommand(r io.Reader) (string, error) {
	br := bufio.NewReader(r)

	count, err := readNulFieldInt(br)
	if err != nil {
		return "", err
	}

	args := make([]string, 0, count)

	for range count {
		arg, err := readNulField(br)
		if err != nil {
			return "", err
		}

		args = append(args, arg)
	}

	return joinRsyncServerCommand(args)
}

func readNulField(r *bufio.Reader) (string, error) {
	field, err := r.ReadString(0)
	if err != nil {
		return "", fmt.Errorf("gabric rsync: malformed rsh arguments: %w", err)
	}

	return strings.TrimSuffix(field, "\x00"), nil
}

func readNulFieldInt(r *bufio.Reader) (int, error) {
	field, err := readNulField(r)
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(field)
	if err != nil || count < 0 {
		return 0, errors.New("gabric rsync: malformed rsh argument count")
	}

	return count, nil
}

// joinRsyncServerCommand drops the rsh "-l user" and host arguments rsync
// prepends and joins the remaining argv into the remote shell command.
func joinRsyncServerCommand(args []string) (string, error) {
	i := 0

	if i < len(args) && args[i] == "-l" {
		i += 2
	}

	if i >= len(args) {
		return "", errors.New("gabric rsync: missing remote host in rsh arguments")
	}

	i++

	if i >= len(args) {
		return "", errors.New("gabric rsync: missing remote command in rsh arguments")
	}

	return strings.Join(args[i:], " "), nil
}
