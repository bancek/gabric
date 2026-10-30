//go:build !unix

package terminal

import "os"

// Resize is unavailable without SIGWINCH.
func Resize(*os.File) (<-chan Size, func()) {
	return nil, func() {}
}
