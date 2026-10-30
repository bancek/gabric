package modules

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// FileStat is the target path's metadata and existence.
type FileStat struct {
	gabric.FileStat
	Exists bool
}

// FileStatData reads the target path metadata as a data node (dry-run safe,
// memoized once per run). A missing path yields Exists=false; other Stat errors
// propagate.
func FileStatData(pathInput input.Input[string]) *reconcile.Node[FileStat] {
	if pathInput == nil {
		panic("modules: FileStatData: nil path")
	}

	return reconcile.Data("modules.file-stat:"+pathInput.Name(), func(ctx context.Context) (FileStat, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return FileStat{}, err
		}

		stat, err := gabric.TargetOps(ctx).Stat(path).RunErr()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return FileStat{}, nil
			}

			return FileStat{}, err
		}

		return FileStat{FileStat: stat, Exists: true}, nil
	}).Node()
}

// FileModeMatchesData reports whether the path exists and its permission bits
// equal mode. A nil or zero mode is not checked. Build it inside Check/Apply for
// a fresh probe, or outside to share one Stat per run.
func FileModeMatchesData(stat *reconcile.Node[FileStat], modeInput input.Input[os.FileMode]) *reconcile.Node[bool] {
	return reconcile.Map(stat, func(ctx context.Context, info FileStat) (bool, error) {
		if modeInput == nil {
			return info.Exists, nil
		}

		mode, err := modeInput.Value(ctx)
		if err != nil {
			return false, err
		}
		if mode == 0 {
			return info.Exists, nil
		}

		return info.Exists && info.Mode.Perm() == mode.Perm(), nil
	})
}

// FileOwnerMatchesData reports whether the path exists and its owner matches user and
// group. A nil or empty user or group is not checked.
func FileOwnerMatchesData(stat *reconcile.Node[FileStat], userInput, groupInput input.Input[string]) *reconcile.Node[bool] {
	return reconcile.Map(stat, func(ctx context.Context, info FileStat) (bool, error) {
		if !info.Exists {
			return false, nil
		}

		user, err := input.Value(ctx, userInput)
		if err != nil {
			return false, err
		}
		if user != "" && info.User != user {
			return false, nil
		}

		group, err := input.Value(ctx, groupInput)
		if err != nil {
			return false, err
		}
		if group != "" && info.Group != group {
			return false, nil
		}

		return true, nil
	})
}
