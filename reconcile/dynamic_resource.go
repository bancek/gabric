package reconcile

import (
	"context"
	"fmt"
)

// DynamicResource builds a resource whose implementation builder is constructed
// when the resource is evaluated. build is read through a data node, so Check
// and Apply share one builder.
//
// The returned builder owns the resource name and report entry. The built
// builder contributes its check, apply, dependencies, invalidations and
// notifications. Its builder name is not used; middleware belongs to the
// returned node.
func DynamicResource[T any](name string, build func(ctx context.Context) (*ResourceBuilder[T], error)) *ResourceBuilder[T] {
	if build == nil {
		panic("reconcile: dynamic resource " + name + ": build is required")
	}

	selected := Data(name+":implementation", func(ctx context.Context) (resourceSpec[T], error) {
		b, err := build(ctx)
		if err != nil {
			return resourceSpec[T]{}, err
		}
		if b == nil || b.apply == nil {
			return resourceSpec[T]{}, fmt.Errorf("reconcile: dynamic resource %q: build returned an invalid builder", name)
		}

		return b.spec(), nil
	}).Node()

	return Resource(name,
		func(ctx context.Context) (bool, error) {
			spec, err := selected.Value(ctx)
			if err != nil {
				return false, err
			}

			return spec.doCheck(ctx)
		},
		func(ctx context.Context) (T, error) {
			spec, err := selected.Value(ctx)
			if err != nil {
				var zero T
				return zero, err
			}

			return spec.doApply(ctx)
		})
}
