package modules

import (
	"context"
	"os"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// DirBuilder builds a node that ensures a target directory exists.
type DirBuilder struct {
	pathInput  input.Input[string]
	modeInput  input.Input[os.FileMode]
	userInput  input.Input[string]
	groupInput input.Input[string]
}

// Dir ensures the target directory at path exists. The node yields bool
// (changed).
func Dir(pathInput input.Input[string]) *DirBuilder {
	return &DirBuilder{pathInput: pathInput}
}

// Mode sets the desired directory mode. A nil or zero mode is not checked.
func (b *DirBuilder) Mode(modeInput input.Input[os.FileMode]) *DirBuilder {
	b.modeInput = modeInput
	return b
}

// User sets the owning user.
func (b *DirBuilder) User(userInput input.Input[string]) *DirBuilder {
	b.userInput = userInput
	return b
}

// Group sets the owning group.
func (b *DirBuilder) Group(groupInput input.Input[string]) *DirBuilder {
	b.groupInput = groupInput
	return b
}

// Build hands off to the reconcile node builder.
func (b *DirBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.pathInput == nil {
		panic("modules: Dir: nil path")
	}

	pathInput, modeInput, userInput, groupInput := b.pathInput, b.modeInput, b.userInput, b.groupInput

	name := "modules.dir:" + pathInput.Name()

	return reconcile.ResourceBool(name,
		func(ctx context.Context) (bool, error) {
			ok, err := DirSatisfiedData(pathInput, modeInput).Value(ctx)
			if err != nil || !ok {
				return false, err
			}
			if userInput == nil && groupInput == nil {
				return true, nil
			}

			return FileOwnerMatchesData(FileStatData(pathInput), userInput, groupInput).Value(ctx)
		},
		func(ctx context.Context) error {
			path, err := pathInput.Value(ctx)
			if err != nil {
				return err
			}
			mode, err := input.Value(ctx, modeInput)
			if err != nil {
				return err
			}
			user, err := input.Value(ctx, userInput)
			if err != nil {
				return err
			}
			group, err := input.Value(ctx, groupInput)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Mkdir(path).
				Recursive().
				Mode(mode).
				User(user).
				Group(group).
				RunErr()
		})
}

// DirSatisfiedData reports whether the target path is a directory with the
// desired mode, as a data node (dry-run safe, memoized once per run). A nil or
// zero mode is not checked.
func DirSatisfiedData(pathInput input.Input[string], modeInput input.Input[os.FileMode]) *reconcile.Node[bool] {
	if pathInput == nil {
		panic("modules: DirSatisfiedData: nil path")
	}

	return reconcile.Data("modules.dir-satisfied:"+pathInput.Name(), func(ctx context.Context) (bool, error) {
		mode, err := input.Value(ctx, modeInput)
		if err != nil {
			return false, err
		}
		if mode == 0 {
			return DirExistsData(pathInput).Value(ctx)
		}

		stat := FileStatData(pathInput)
		info, err := stat.Value(ctx)
		if err != nil {
			return false, err
		}
		if !info.Exists || !info.Mode.IsDir() {
			return false, nil
		}

		return FileModeMatchesData(stat, input.Static(mode)).Value(ctx)
	}).Node()
}
