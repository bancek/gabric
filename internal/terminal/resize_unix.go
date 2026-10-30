//go:build unix

package terminal

import (
	"os"
	"os/signal"
	"syscall"
)

// Resize reports terminal size changes on f until stop is called.
func Resize(f *os.File) (<-chan Size, func()) {
	sizes := make(chan Size, 1)
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(sigs, syscall.SIGWINCH)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-sigs:
				if size, ok := GetSize(f); ok {
					select {
					case sizes <- size:
					default:
					}
				}
			}
		}
	}()

	stop := func() {
		signal.Stop(sigs)
		close(done)
	}

	return sizes, stop
}
