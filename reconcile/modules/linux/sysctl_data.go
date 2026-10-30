package linux

import (
	"context"
	"fmt"
	"strings"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// SysctlSatisfiedData reports whether the runtime value and persisted drop-in
// match k=v, as a data node (dry-run safe, memoized once per run).
func SysctlSatisfiedData(keyInput, valueInput input.Input[string]) *reconcile.Node[bool] {
	if keyInput == nil {
		panic("linux: SysctlSatisfiedData: nil key")
	}
	if valueInput == nil {
		panic("linux: SysctlSatisfiedData: nil value")
	}

	return reconcile.Data("linux.sysctl-satisfied:"+keyInput.Name(), func(ctx context.Context) (bool, error) {
		key, err := keyInput.Value(ctx)
		if err != nil {
			return false, err
		}
		value, err := valueInput.Value(ctx)
		if err != nil {
			return false, err
		}
		content := fmt.Sprintf("%s = %s\n", key, value)

		runtime, err := SysctlRuntimeData(input.Static(key)).Value(ctx)
		if err != nil {
			return false, err
		}
		if runtime != value {
			return false, nil
		}

		dropin, err := SysctlDropinData(input.Static(key)).Value(ctx)
		if err != nil {
			return false, err
		}

		return dropin == content, nil
	}).Node()
}

// SysctlRuntimeData reads the runtime sysctl value as a data node (dry-run safe,
// memoized once per run). It returns "" when the value cannot be read; other
// errors propagate.
func SysctlRuntimeData(keyInput input.Input[string]) *reconcile.Node[string] {
	if keyInput == nil {
		panic("linux: SysctlRuntimeData: nil key")
	}

	return reconcile.Data("linux.sysctl-runtime:"+keyInput.Name(), func(ctx context.Context) (string, error) {
		key, err := keyInput.Value(ctx)
		if err != nil {
			return "", err
		}
		procPath := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")

		out, err := modules.CommandOutputData(input.List("cat", procPath)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return "", err
			}

			return "", nil
		}

		return strings.TrimSpace(out), nil
	}).Node()
}

// SysctlDropinData reads the persisted sysctl drop-in as a data node (dry-run
// safe, memoized once per run). It returns "" when the file cannot be read;
// other errors propagate.
func SysctlDropinData(keyInput input.Input[string]) *reconcile.Node[string] {
	if keyInput == nil {
		panic("linux: SysctlDropinData: nil key")
	}

	return reconcile.Data("linux.sysctl-dropin:"+keyInput.Name(), func(ctx context.Context) (string, error) {
		key, err := keyInput.Value(ctx)
		if err != nil {
			return "", err
		}
		path := sysctlDropinPath(key)

		out, err := modules.CommandOutputData(input.List("cat", path)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return "", err
			}

			return "", nil
		}

		return out, nil
	}).Node()
}
