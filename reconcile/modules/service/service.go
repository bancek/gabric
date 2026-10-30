package service

import (
	"context"
	"fmt"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

// ServiceBuilder builds a node that ensures a service's enable/active state
// using the target's init system.
type ServiceBuilder struct {
	nameInput    input.Input[string]
	enabledInput input.Input[bool]
	startedInput input.Input[bool]
}

// Service ensures the service name is enabled.
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

// Build hands off to the reconcile resource builder.
func (b *ServiceBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("service: Service: nil name")
	}

	nameInput, enabledInput, startedInput := b.nameInput, b.enabledInput, b.startedInput
	managerData := ManagerData()

	return reconcile.DynamicResource("service.service:"+nameInput.Name(), func(ctx context.Context) (*reconcile.ResourceBuilder[bool], error) {
		manager, err := managerData.Value(ctx)
		if err != nil {
			return nil, err
		}

		switch manager {
		case ManagerSystemd:
			s := systemd.Service(nameInput)
			if enabledInput != nil {
				s = s.Enabled(enabledInput)
			}
			if startedInput != nil {
				s = s.Started(startedInput)
			}

			return s.Build(), nil
		default:
			return nil, fmt.Errorf("service: unsupported init system")
		}
	})
}
