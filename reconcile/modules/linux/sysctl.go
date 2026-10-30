package linux

import (
	"context"
	"fmt"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// Sysctl ensures the kernel parameter k has the value v and persists it in a
// drop-in under /etc/sysctl.d. The node yields bool (changed).
func Sysctl(keyInput, valueInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	if keyInput == nil {
		panic("linux: Sysctl: nil key")
	}
	if valueInput == nil {
		panic("linux: Sysctl: nil value")
	}

	return reconcile.ResourceBool("linux.sysctl:"+keyInput.Name(),
		func(ctx context.Context) (bool, error) {
			return SysctlSatisfiedData(keyInput, valueInput).Value(ctx)
		},
		func(ctx context.Context) error {
			key, err := keyInput.Value(ctx)
			if err != nil {
				return err
			}
			value, err := valueInput.Value(ctx)
			if err != nil {
				return err
			}
			dropinPath := sysctlDropinPath(key)
			content := fmt.Sprintf("%s = %s\n", key, value)

			if err := gabric.TargetOps(ctx).Put(dropinPath).EnsureParent().String(content).RunErr(); err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("sysctl", "-p", dropinPath).RunErr()
		})
}

func sysctlDropinPath(k string) string {
	return "/etc/sysctl.d/99-reconcile-" + strings.ReplaceAll(k, ".", "-") + ".conf"
}
