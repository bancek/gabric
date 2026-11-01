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
	userInput    input.Input[bool]
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

// User sets whether the service is managed by the systemd user manager
// (systemctl --user). Calling it with no argument means true.
func (b *ServiceBuilder) User(flag ...input.Input[bool]) *ServiceBuilder {
	b.userInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile node builder.
func (b *ServiceBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("systemd: Service: nil name")
	}

	nameInput, enabledInput, startedInput, userInput := b.nameInput, b.enabledInput, b.startedInput, b.userInput

	return reconcile.ResourceBool("systemd.service:"+nameInput.Name(),
		func(ctx context.Context) (bool, error) {
			user, err := input.Value(ctx, userInput)
			if err != nil {
				return false, err
			}
			if user {
				return UserSatisfiedData(nameInput, enabledInput, startedInput).Value(ctx)
			}

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
			user, err := input.Value(ctx, userInput)
			if err != nil {
				return err
			}
			if !enabled && started {
				return fmt.Errorf("systemd: service %s: started requires enabled", name)
			}

			return enable(ctx, name, enabled, started, user)
		})
}

// SatisfiedData reports whether the system service matches the desired
// enable/active state.
func SatisfiedData(nameInput input.Input[string], enabledInput, startedInput input.Input[bool]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: SatisfiedData: nil name")
	}

	return satisfiedData(nameInput, enabledInput, startedInput, false)
}

// UserSatisfiedData reports whether the user service matches the desired
// enable/active state.
func UserSatisfiedData(nameInput input.Input[string], enabledInput, startedInput input.Input[bool]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("systemd: UserSatisfiedData: nil name")
	}

	return satisfiedData(nameInput, enabledInput, startedInput, true)
}

func satisfiedData(nameInput input.Input[string], enabledInput, startedInput input.Input[bool], user bool) *reconcile.Node[bool] {
	nodeName := "systemd.satisfied:" + nameInput.Name()
	if user {
		nodeName = "systemd.user-satisfied:" + nameInput.Name()
	}

	return reconcile.Data(nodeName, func(ctx context.Context) (bool, error) {
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

		actualEnabled, err := enabledData(input.Static(name), user).Value(ctx)
		if err != nil {
			return false, err
		}
		if actualEnabled != enabled {
			return false, nil
		}

		if enabled && started {
			actualActive, err := activeData(input.Static(name), user).Value(ctx)
			if err != nil {
				return false, err
			}
			if !actualActive {
				return false, nil
			}
		}

		if !enabled {
			actualActive, err := activeData(input.Static(name), user).Value(ctx)
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

func enable(ctx context.Context, name string, enabled, started, user bool) error {
	ops := gabric.TargetOps(ctx)

	switch {
	case !enabled:
		return ops.Exec(systemctlArgs(user, "disable", "--now", name)...).RunErr()
	case started:
		return ops.Exec(systemctlArgs(user, "enable", "--now", name)...).RunErr()
	default:
		return ops.Exec(systemctlArgs(user, "enable", name)...).RunErr()
	}
}

func systemctlArgs(user bool, args ...string) []string {
	if user {
		return append([]string{"systemctl", "--user"}, args...)
	}

	return append([]string{"systemctl"}, args...)
}
