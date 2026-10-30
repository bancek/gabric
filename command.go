package gabric

import "github.com/bancek/gabric/internal/command"

// Command is a parsed command recorded by the test harness.
type Command = command.Command

// Sudo describes the sudo context a command ran under. A nil *Sudo means no
// sudo.
type Sudo = command.Sudo
