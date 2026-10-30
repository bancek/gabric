package systemd

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// EnabledData reports whether the systemd service name is enabled, as a data
// node (dry-run safe, memoized once per run). A disabled unit is false, not an
// error.
func EnabledData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: EnabledData: nil name")
	}

	return reconcile.Data("systemd.enabled:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		e := gabric.TargetOps(ctx).Exec("systemctl", "is-enabled", name).Capture()
		err = e.RunErr()
		status := strings.TrimSpace(e.StdoutStr())
		if err != nil && !modules.IsExitStatus(err, 1, 4) {
			return false, err
		}
		if err != nil && status == "" {
			return false, err
		}

		return status == "enabled", nil
	}).Node()
}

// ActiveData reports whether the systemd service name is active, as a data node
// (dry-run safe, memoized once per run). An inactive unit is false, not an
// error.
func ActiveData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: ActiveData: nil name")
	}

	return reconcile.Data("systemd.active:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		e := gabric.TargetOps(ctx).Exec("systemctl", "is-active", name).Capture()
		err = e.RunErr()
		status := strings.TrimSpace(e.StdoutStr())
		if err != nil && !modules.IsExitStatus(err, 3, 4) {
			return false, err
		}
		if err != nil && status == "" {
			return false, err
		}

		return status == "active", nil
	}).Node()
}
