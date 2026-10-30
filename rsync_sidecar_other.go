//go:build !unix

package gabric

import "os"

// shutdownWrite is a no-op on platforms without the native rsync transport.
func shutdownWrite(*os.File) {}
