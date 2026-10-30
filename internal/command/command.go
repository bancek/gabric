// Package command defines the parsed command records shared by the shell test
// core and the public test harness.
package command

import "slices"

// Sudo describes the sudo context a command ran under. A nil *Sudo means no
// sudo.
type Sudo struct {
	// User is the sudo target user. Empty means sudo's default, normally root.
	User string
	// Flags are passed to sudo before -u. Use User for -u.
	Flags []string
}

// Equal reports whether s and other describe the same sudo context.
func (s *Sudo) Equal(other *Sudo) bool {
	if s == nil || other == nil {
		return s == nil && other == nil
	}

	return s.User == other.User && slices.Equal(s.Flags, other.Flags)
}

// Prefix returns the sudo command prefix for s: "sudo", then Flags, then
// "-u User" when User is set. A nil s has no prefix.
func (s *Sudo) Prefix() []string {
	if s == nil {
		return nil
	}

	args := make([]string, 0, 2+len(s.Flags))
	args = append(args, "sudo")
	args = append(args, s.Flags...)

	if s.User != "" {
		args = append(args, "-u", s.User)
	}

	return args
}

// WrapCommand returns cmd prefixed with s.Prefix(). A nil s returns cmd
// unchanged.
func (s *Sudo) WrapCommand(cmd []string) []string {
	if s == nil {
		return cmd
	}

	prefix := s.Prefix()

	out := make([]string, 0, len(prefix)+len(cmd))
	out = append(out, prefix...)
	out = append(out, cmd...)

	return out
}

// Command is a parsed command recorded by the shell test core.
type Command struct {
	Name string
	Args []string
	Dir  string
	Sudo *Sudo
	// Env is the command's exec environment overlay.
	Env []string
}

// HasArg reports whether arg is one of the command's arguments.
func (c Command) HasArg(arg string) bool {
	return slices.Contains(c.Args, arg)
}

// Equal reports whether c and other describe the same command.
func (c Command) Equal(other Command) bool {
	return c.Name == other.Name &&
		c.Dir == other.Dir &&
		slices.Equal(c.Args, other.Args) &&
		c.Sudo.Equal(other.Sudo) &&
		slices.Equal(c.Env, other.Env)
}

func (c Command) WithEnv(env ...string) Command {
	c.Env = env
	return c
}
