package systemd

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// EnabledData reports whether the system service name is enabled, as a data
// node (dry-run safe, memoized once per run). A disabled unit is false, not an
// error.
func EnabledData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: EnabledData: nil name")
	}

	return enabledData(nameInput, false)
}

// UserEnabledData reports whether the user service name is enabled, as a data
// node (dry-run safe, memoized once per run). A disabled unit is false, not an
// error.
func UserEnabledData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: UserEnabledData: nil name")
	}

	return enabledData(nameInput, true)
}

func enabledData(nameInput input.Input[string], user bool) *reconcile.Node[bool] {
	nodeName := "systemd.enabled:" + nameInput.Name()
	if user {
		nodeName = "systemd.user-enabled:" + nameInput.Name()
	}

	return reconcile.Data(nodeName, func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		e := gabric.TargetOps(ctx).Exec(systemctlArgs(user, "is-enabled", name)...).Capture()
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

// ActiveData reports whether the system service name is active, as a data node
// (dry-run safe, memoized once per run). An inactive unit is false, not an
// error.
func ActiveData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: ActiveData: nil name")
	}

	return activeData(nameInput, false)
}

// UserActiveData reports whether the user service name is active, as a data
// node (dry-run safe, memoized once per run). An inactive unit is false, not an
// error.
func UserActiveData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: UserActiveData: nil name")
	}

	return activeData(nameInput, true)
}

func activeData(nameInput input.Input[string], user bool) *reconcile.Node[bool] {
	nodeName := "systemd.active:" + nameInput.Name()
	if user {
		nodeName = "systemd.user-active:" + nameInput.Name()
	}

	return reconcile.Data(nodeName, func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		e := gabric.TargetOps(ctx).Exec(systemctlArgs(user, "is-active", name)...).Capture()
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
