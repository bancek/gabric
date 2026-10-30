//go:build unix

package gabric

import (
	"os"
	"syscall"
)

// shutdownWrite half-closes the write side of f so the gabric process sees
// EOF on the data socketpair while the sidecar keeps reading the response.
func shutdownWrite(f *os.File) {
	_ = syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
}
