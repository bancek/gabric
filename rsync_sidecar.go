package gabric

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
)

// rsyncSidecarArg is the reserved first argument that turns the gabric binary
// into the rsync remote shell sidecar. The local rsync process is started with
// --rsh set to the current executable plus this argument; rsync then appends
// the host and the remote server command as further arguments.
//
// The sidecar only proxies the rsync protocol between its standard input and
// output and the file descriptors provided by the gabric process that started
// local rsync. It does not implement SSH: that stays in the gabric process, so
// the remote transport is fully customizable.
const rsyncSidecarArg = "gabric-rsync-rsh"

const (
	rsyncDataFD   = 3
	rsyncArgsFD   = 4
	rsyncStatusFD = 5
)

func init() {
	if len(os.Args) >= 2 && os.Args[1] == rsyncSidecarArg {
		os.Exit(runRsyncSidecar(os.Args[2:]))
	}
}

// runRsyncSidecar bridges rsync's stdin/stdout to the inherited transport file
// descriptors and exits with the remote command's status:
//
//	fd 3 - data socketpair (rsync protocol, both directions)
//	fd 4 - argument pipe (the NUL framed argv from rsync)
//	fd 5 - status pipe (the remote exit status)
func runRsyncSidecar(args []string) int {
	data := os.NewFile(rsyncDataFD, "gabric-rsync-data")
	argsOut := os.NewFile(rsyncArgsFD, "gabric-rsync-args")
	statusIn := os.NewFile(rsyncStatusFD, "gabric-rsync-status")

	if data == nil || argsOut == nil || statusIn == nil {
		fmt.Fprintln(os.Stderr, "gabric rsync sidecar: transport file descriptors not available")
		return 1
	}

	if err := writeRsyncArgs(argsOut, args); err != nil {
		fmt.Fprintln(os.Stderr, "gabric rsync sidecar:", err)
		return 1
	}

	argsOut.Close()

	go func() {
		io.Copy(data, os.Stdin)
		shutdownWrite(data)
	}()

	io.Copy(os.Stdout, data)

	return readRsyncStatus(statusIn)
}

func writeRsyncArgs(w io.Writer, args []string) error {
	var b bytes.Buffer

	fmt.Fprintf(&b, "%d", len(args))
	b.WriteByte(0)

	for _, arg := range args {
		b.WriteString(arg)
		b.WriteByte(0)
	}

	_, err := w.Write(b.Bytes())

	return err
}

func readRsyncStatus(r io.Reader) int {
	b, err := io.ReadAll(r)
	if err != nil {
		return 1
	}

	status, err := strconv.Atoi(string(bytes.TrimSpace(b)))
	if err != nil || status < 0 {
		return 1
	}

	return status
}
