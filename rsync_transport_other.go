//go:build !unix

package gabric

import (
	"os"
)

// newSocketPair is unavailable on non-unix platforms; RsyncOps falls back to
// the system ssh transport.
func newSocketPair() (parent *os.File, child *os.File, err error) {
	return nil, nil, errRsyncTransportUnsupported
}
