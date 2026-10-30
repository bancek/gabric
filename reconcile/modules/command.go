package modules

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// Command reconciles a command with no probe: it runs cmd on the target host
// (inside Do, so a dry run skips it). The node is named
// "modules.command:<cmd>".
func Command(cmdInput input.Input[[]string]) *reconcile.ResourceBuilder[bool] {
	if cmdInput == nil {
		panic("modules: Command: nil command")
	}

	return reconcile.ResourceBool("modules.command:"+cmdInput.Name(), nil, func(ctx context.Context) error {
		cmd, err := cmdInput.Value(ctx)
		if err != nil {
			return err
		}

		return gabric.TargetOps(ctx).Exec(cmd...).RunErr()
	})
}
