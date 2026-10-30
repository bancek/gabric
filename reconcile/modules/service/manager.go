package service

import (
	"context"
	"fmt"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// Manager identifies the target init system.
type Manager int

const (
	// ManagerUnknown is an unsupported init system.
	ManagerUnknown Manager = iota
	// ManagerSystemd is systemd.
	ManagerSystemd
)

// ManagerData reads the target init system as a data node (dry-run safe,
// memoized once per run).
func ManagerData() *reconcile.Node[Manager] {
	return reconcile.Data("service.manager", func(ctx context.Context) (Manager, error) {
		systemd, err := modules.PathExistsData(input.Static("/run/systemd/system")).Value(ctx)
		if err != nil {
			return ManagerUnknown, err
		}
		if systemd {
			return ManagerSystemd, nil
		}

		return ManagerUnknown, fmt.Errorf("service: unsupported init system")
	}).Node()
}
