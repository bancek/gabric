package modules

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// CommandOutputData runs cmd on the target and yields its captured stdout as a
// data node (dry-run safe, memoized once per run). Use only side-effect-free
// commands: data nodes run in a dry run.
func CommandOutputData(cmdInput input.Input[[]string]) *reconcile.Node[string] {
	if cmdInput == nil {
		panic("modules: CommandOutputData: nil command")
	}

	return reconcile.Data("modules.command-output:"+cmdInput.Name(), func(ctx context.Context) (string, error) {
		cmd, err := cmdInput.Value(ctx)
		if err != nil {
			return "", err
		}

		e := gabric.TargetOps(ctx).Exec(cmd...).Capture()
		err = e.RunErr()

		return e.StdoutStr(), err
	}).Node()
}

// CommandStderrData runs cmd on the target and yields its captured stderr as a
// data node (dry-run safe, memoized once per run). Some tools (for example
// docker compose) log progress to stderr. Use only side-effect-free commands:
// data nodes run in a dry run.
func CommandStderrData(cmdInput input.Input[[]string]) *reconcile.Node[string] {
	if cmdInput == nil {
		panic("modules: CommandStderrData: nil command")
	}

	return reconcile.Data("modules.command-stderr:"+cmdInput.Name(), func(ctx context.Context) (string, error) {
		cmd, err := cmdInput.Value(ctx)
		if err != nil {
			return "", err
		}

		e := gabric.TargetOps(ctx).Exec(cmd...).Capture()
		err = e.RunErr()

		return e.StderrStr(), err
	}).Node()
}
