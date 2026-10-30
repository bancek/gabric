package gabrictest

import (
	"github.com/bancek/gabric"
)

// Command is the parsed command record used by the harness.
type Command = gabric.Command

// Sudo describes the sudo context a command ran under. A nil *Sudo means no
// sudo.
type Sudo = gabric.Sudo

// Result is the result of a handled command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Handler runs a command registered with Side.Handle.
type Handler func(cmd Command) Result
