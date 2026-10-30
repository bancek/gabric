package shelltest

import (
	"context"
	"slices"
	"sync"

	"github.com/bancek/gabric/internal/command"
	"mvdan.cc/sh/v3/interp"
)

// Command is a parsed command recorded by the core.
type Command = command.Command

// Sudo is the sudo context a command ran under. A nil *Sudo means no sudo.
type Sudo = command.Sudo

// Recorder stores commands in execution order.
type Recorder struct {
	mu   sync.Mutex
	cmds []Command
}

// Record appends cmd.
func (r *Recorder) Record(cmd Command) {
	cmd.Args = slices.Clone(cmd.Args)
	cmd.Env = slices.Clone(cmd.Env)

	if cmd.Sudo != nil {
		sudo := *cmd.Sudo
		sudo.Flags = slices.Clone(sudo.Flags)
		cmd.Sudo = &sudo
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cmds = append(r.cmds, cmd)
}

// Commands returns a copy of the recorded commands.
func (r *Recorder) Commands() []Command {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmds := make([]Command, len(r.cmds))

	for i, cmd := range r.cmds {
		cmd.Args = slices.Clone(cmd.Args)
		cmd.Env = slices.Clone(cmd.Env)

		if cmd.Sudo != nil {
			sudo := *cmd.Sudo
			sudo.Flags = slices.Clone(sudo.Flags)
			cmd.Sudo = &sudo
		}

		cmds[i] = cmd
	}

	return cmds
}

// Reset removes the recorded commands.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cmds = nil
}

// Commands returns a copy of the commands recorded by the core.
func (c *Core) Commands() []Command {
	return c.recorder.Commands()
}

// Reset removes the commands recorded by the core.
func (c *Core) Reset() {
	c.recorder.Reset()
}

func (c *Core) callHandler(ctx context.Context, args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	if handler, ok := c.handler(args[0]); ok && handler.Transparent {
		return args, nil
	}

	c.recorder.Record(Command{
		Name: args[0],
		Args: args[1:],
		Dir:  virtualDir(interp.HandlerCtx(ctx).Dir),
		Sudo: sudoFromContext(ctx),
		Env:  envFromContext(ctx),
	})

	return args, nil
}
