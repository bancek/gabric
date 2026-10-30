package docker

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// NetworkExistsData reports whether the docker network exists, as a data
// node (dry-run safe, memoized once per run). An inspect "not found" exit is
// false; other errors propagate.
func NetworkExistsData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("docker: NetworkExistsData: nil name")
	}

	return reconcile.Data("docker.network-exists:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		_, err = modules.CommandOutputData(input.List("docker", "network", "inspect", name)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return false, err
			}

			return false, nil
		}

		return true, nil
	}).Node()
}

// Network ensures a docker network exists. The node yields bool
// (changed).
func Network(nameInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	if nameInput == nil {
		panic("docker: Network: nil name")
	}

	return reconcile.ResourceBool("docker.network:"+nameInput.Name(),
		func(ctx context.Context) (bool, error) {
			return NetworkExistsData(nameInput).Value(ctx)
		},
		func(ctx context.Context) error {
			name, err := nameInput.Value(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("docker", "network", "create", name).RunErr()
		})
}
