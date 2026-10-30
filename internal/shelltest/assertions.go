package shelltest

import (
	"slices"
	"testing"
)

// CommandsProvider returns the commands recorded so far.
type CommandsProvider func() []Command

// Assertions checks commands recorded by a core or harness.
type Assertions struct {
	t        *testing.T
	commands CommandsProvider
}

// NewAssertions returns assertions over commands.
func NewAssertions(t *testing.T, commands CommandsProvider) *Assertions {
	return &Assertions{t: t, commands: commands}
}

// Find returns the first command with name and exactly args.
func (a *Assertions) Find(name string, args ...string) (Command, bool) {
	return a.FindFunc(func(cmd Command) bool {
		return cmd.Name == name && slices.Equal(cmd.Args, args)
	})
}

// FindFunc returns the first command matching match.
func (a *Assertions) FindFunc(match func(Command) bool) (Command, bool) {
	for _, cmd := range a.commands() {
		if match(cmd) {
			return cmd, true
		}
	}

	return Command{}, false
}

// AssertCommands fails unless the recorded commands equal want in order.
func (a *Assertions) AssertCommands(want ...Command) {
	a.t.Helper()

	if got := a.commands(); !slices.EqualFunc(got, want, Command.Equal) {
		a.t.Errorf("commands = %v, want %v", got, want)
	}
}

// AssertExecuted fails unless name was executed with exactly args. It returns
// the matching command.
func (a *Assertions) AssertExecuted(name string, args ...string) Command {
	a.t.Helper()

	cmd, ok := a.Find(name, args...)
	if !ok {
		a.t.Errorf("command not executed: %s %v; recorded: %v", name, args, a.commands())
	}

	return cmd
}

// AssertNotExecuted fails when name was executed with exactly args.
func (a *Assertions) AssertNotExecuted(name string, args ...string) {
	a.t.Helper()

	if _, ok := a.Find(name, args...); ok {
		a.t.Errorf("command executed: %s %v; recorded: %v", name, args, a.commands())
	}
}

// AssertFunc fails unless a command matches match.
func (a *Assertions) AssertFunc(match func(Command) bool, msg string) {
	a.t.Helper()

	if _, ok := a.FindFunc(match); !ok {
		a.t.Errorf("%s; recorded: %v", msg, a.commands())
	}
}
