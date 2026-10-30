package systemd

import (
	"context"
	"fmt"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// ServiceBuilder builds a node that ensures a systemd service's enable/active
// state.
type ServiceBuilder struct {
	nameInput    input.Input[string]
	enabledInput input.Input[bool]
	startedInput input.Input[bool]
}

// Service ensures the systemd service name is enabled.
func Service(nameInput input.Input[string]) *ServiceBuilder {
	return &ServiceBuilder{nameInput: nameInput}
}

// Enabled sets whether the service is enabled.
func (b *ServiceBuilder) Enabled(flag ...input.Input[bool]) *ServiceBuilder {
	b.enabledInput = input.Flag(flag...)
	return b
}

// Started also ensures the service is active (enable --now).
func (b *ServiceBuilder) Started(flag ...input.Input[bool]) *ServiceBuilder {
	b.startedInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile node builder.
func (b *ServiceBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("systemd: Service: nil name")
	}

	nameInput, enabledInput, startedInput := b.nameInput, b.enabledInput, b.startedInput

	return reconcile.ResourceBool("systemd.service:"+nameInput.Name(),
		func(ctx context.Context) (bool, error) {
			return SatisfiedData(nameInput, enabledInput, startedInput).Value(ctx)
		},
		func(ctx context.Context) error {
			name, err := nameInput.Value(ctx)
			if err != nil {
				return err
			}
			enabled, err := input.ValueOr(ctx, enabledInput, true)
			if err != nil {
				return err
			}
			started, err := input.Value(ctx, startedInput)
			if err != nil {
				return err
			}
			if !enabled && started {
				return fmt.Errorf("systemd: service %s: started requires enabled", name)
			}

			return enable(ctx, name, enabled, started)
		})
}

// SatisfiedData reports whether the systemd service matches the desired
// enable/active state.
func SatisfiedData(nameInput input.Input[string], enabledInput, startedInput input.Input[bool]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: SatisfiedData: nil name")
	}

	return reconcile.Data("systemd.satisfied:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}
		enabled, err := input.ValueOr(ctx, enabledInput, true)
		if err != nil {
			return false, err
		}
		started, err := input.Value(ctx, startedInput)
		if err != nil {
			return false, err
		}
		if !enabled && started {
			return false, fmt.Errorf("systemd: service %s: started requires enabled", name)
		}

		actualEnabled, err := EnabledData(input.Static(name)).Value(ctx)
		if err != nil {
			return false, err
		}
		if actualEnabled != enabled {
			return false, nil
		}

		if enabled && started {
			actualActive, err := ActiveData(input.Static(name)).Value(ctx)
			if err != nil {
				return false, err
			}
			if !actualActive {
				return false, nil
			}
		}

		if !enabled {
			actualActive, err := ActiveData(input.Static(name)).Value(ctx)
			if err != nil {
				return false, err
			}
			if actualActive {
				return false, nil
			}
		}

		return true, nil
	}).Node()
}

func enable(ctx context.Context, name string, enabled, started bool) error {
	ops := gabric.TargetOps(ctx)

	switch {
	case !enabled:
		return ops.Exec("systemctl", "disable", "--now", name).RunErr()
	case started:
		return ops.Exec("systemctl", "enable", "--now", name).RunErr()
	default:
		return ops.Exec("systemctl", "enable", name).RunErr()
	}
}
