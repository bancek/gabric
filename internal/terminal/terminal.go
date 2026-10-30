package terminal

import (
	"os"

	"golang.org/x/term"
)

// Size is the terminal size in character cells.
type Size struct {
	Width  int
	Height int
}

// IsTerminal reports whether f is a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// GetSize returns the terminal size in character cells.
func GetSize(f *os.File) (Size, bool) {
	width, height, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return Size{}, false
	}

	return Size{Width: width, Height: height}, true
}

// MakeRaw puts the terminal into raw mode and returns a function that restores
// the previous state.
func MakeRaw(f *os.File) (restore func(), err error) {
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return nil, err
	}

	return func() {
		term.Restore(int(f.Fd()), state)
	}, nil
}
