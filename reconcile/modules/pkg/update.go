package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/apk"
	"github.com/bancek/gabric/reconcile/modules/apt"
)

// UpdateBuilder builds a node that refreshes the target package index.
type UpdateBuilder struct {
	maxAgeInput input.Input[time.Duration]
}

// Update refreshes the target package index using the selected package manager.
// Without MaxAge it always runs when evaluated.
func Update() *UpdateBuilder {
	return &UpdateBuilder{}
}

// MaxAge refreshes the index only when the target package cache is older than
// maxAge. A nil or non-positive duration keeps the default behavior and always
// refreshes.
func (b *UpdateBuilder) MaxAge(maxAgeInput input.Input[time.Duration]) *UpdateBuilder {
	next := *b
	next.maxAgeInput = maxAgeInput
	return &next
}

// Build hands off to the reconcile node builder.
func (b *UpdateBuilder) Build() *reconcile.ResourceBuilder[bool] {
	maxAgeInput := b.maxAgeInput
	managerData := ManagerData()

	return reconcile.DynamicResource("pkg.update", func(ctx context.Context) (*reconcile.ResourceBuilder[bool], error) {
		manager, err := managerData.Value(ctx)
		if err != nil {
			return nil, err
		}

		switch manager {
		case ManagerApt:
			return apt.Update().MaxAge(maxAgeInput).Build(), nil
		case ManagerApk:
			return apk.Update().MaxAge(maxAgeInput).Build(), nil
		default:
			return nil, fmt.Errorf("pkg: unsupported package manager")
		}
	})
}
