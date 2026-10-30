package pkg

import (
	"context"
	"fmt"

	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/linux"
)

// Manager identifies the target package manager.
type Manager int

const (
	// ManagerUnknown is an unsupported package manager.
	ManagerUnknown Manager = iota
	// ManagerApt is the Debian/Ubuntu apt package manager.
	ManagerApt
	// ManagerApk is the Alpine apk package manager.
	ManagerApk
)

// ManagerData reads the target package manager as a data node (dry-run safe,
// memoized once per run).
func ManagerData() *reconcile.Node[Manager] {
	releaseData := linux.OSReleaseData()

	return reconcile.Data("pkg.manager", func(ctx context.Context) (Manager, error) {
		release, err := releaseData.Value(ctx)
		if err != nil {
			return ManagerUnknown, err
		}

		switch {
		case release.IsLike("debian", "ubuntu"):
			return ManagerApt, nil
		case release.IsLike("alpine"):
			return ManagerApk, nil
		default:
			return ManagerUnknown, fmt.Errorf("pkg: unsupported distribution %q", release.ID)
		}
	}).Node()
}
