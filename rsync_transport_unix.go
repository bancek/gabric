//go:build unix

package gabric

import (
	"os"
	"syscall"
)

// newSocketPair returns a connected, full duplex socket pair. The fds are
// non-blocking so Go's runtime poller manages them: that lets os.File.Close
// unblock concurrent reads (for example the SSH session's stdin copier).
func newSocketPair() (parent *os.File, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}

	for _, fd := range fds {
		syscall.CloseOnExec(fd)

		if err := syscall.SetNonblock(fd, true); err != nil {
			syscall.Close(fds[0])
			syscall.Close(fds[1])
			return nil, nil, err
		}
	}

	parent = os.NewFile(uintptr(fds[0]), "gabric-rsync-data")
	child = os.NewFile(uintptr(fds[1]), "gabric-rsync-data")

	return parent, child, nil
}
