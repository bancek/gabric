package modules

import (
	"context"
	"os"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// FileBuilder builds a node that ensures a target file has the given content,
// mode and owner.
type FileBuilder struct {
	pathInput       input.Input[string]
	contentInput    input.Input[[]byte]
	modeInput       input.Input[os.FileMode]
	userInput       input.Input[string]
	groupInput      input.Input[string]
	createOnlyInput input.Input[bool]
}

// File ensures the target file at path matches the desired content and mode. The
// node's value reports whether it changed.
func File(pathInput input.Input[string]) *FileBuilder {
	return &FileBuilder{pathInput: pathInput}
}

// Content sets the desired content.
func (b *FileBuilder) Content(contentInput input.Input[[]byte]) *FileBuilder {
	b.contentInput = contentInput
	return b
}

// Mode sets the desired file mode. A nil or zero mode is not checked.
func (b *FileBuilder) Mode(modeInput input.Input[os.FileMode]) *FileBuilder {
	b.modeInput = modeInput
	return b
}

// User sets the owning user.
func (b *FileBuilder) User(userInput input.Input[string]) *FileBuilder {
	b.userInput = userInput
	return b
}

// Group sets the owning group.
func (b *FileBuilder) Group(groupInput input.Input[string]) *FileBuilder {
	b.groupInput = groupInput
	return b
}

// CreateOnly only writes the file when it does not already exist.
func (b *FileBuilder) CreateOnly(flag ...input.Input[bool]) *FileBuilder {
	b.createOnlyInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile node builder.
func (b *FileBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.pathInput == nil {
		panic("modules: File: nil path")
	}
	if b.contentInput == nil {
		panic("modules: File: nil content")
	}

	pathInput, contentInput, modeInput, userInput, groupInput := b.pathInput, b.contentInput, b.modeInput, b.userInput, b.groupInput
	createOnlyInput := b.createOnlyInput
	if createOnlyInput == nil {
		createOnlyInput = input.False
	}

	name := "modules.file:" + pathInput.Name()

	return reconcile.ResourceBool(name,
		func(ctx context.Context) (bool, error) {
			ok, err := FileSatisfiedData(pathInput, contentInput, modeInput, createOnlyInput).Value(ctx)
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
			content, err := contentInput.Value(ctx)
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

			return gabric.TargetOps(ctx).Put(path).
				EnsureParent().
				Bytes(content).
				Mode(mode).
				User(user).
				Group(group).
				RunErr()
		})
}

// FileSatisfiedData reports whether the target file matches content and mode, as
// a data node (dry-run safe, memoized once per run). createOnly is satisfied
// whenever the file exists, whatever its content. A nil mode is not checked.
func FileSatisfiedData(
	pathInput input.Input[string],
	contentInput input.Input[[]byte],
	modeInput input.Input[os.FileMode],
	createOnlyInput input.Input[bool],
) *reconcile.Node[bool] {
	if pathInput == nil {
		panic("modules: FileSatisfiedData: nil path")
	}
	if contentInput == nil {
		panic("modules: FileSatisfiedData: nil content")
	}

	return reconcile.Data("modules.file-satisfied:"+pathInput.Name(), func(ctx context.Context) (bool, error) {
		createOnly, err := input.Value(ctx, createOnlyInput)
		if err != nil {
			return false, err
		}

		if createOnly {
			return FileExistsData(pathInput).Value(ctx)
		}

		ok, err := FileContentMatchesData(FileHashData(pathInput), contentInput).Value(ctx)
		if err != nil || !ok {
			return false, err
		}

		if modeInput == nil {
			return true, nil
		}

		return FileModeMatchesData(FileStatData(pathInput), modeInput).Value(ctx)
	}).Node()
}

// FileMode returns a static file mode input. It lets call sites pass an
// untyped mode constant, for example modules.FileMode(0o644).
func FileMode(mode os.FileMode) input.Input[os.FileMode] {
	return input.Static(mode)
}
