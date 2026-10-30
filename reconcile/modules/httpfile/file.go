package httpfile

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// FileBuilder builds a node that downloads a URL to a target file.
type FileBuilder struct {
	urlInput        input.Input[string]
	pathInput       input.Input[string]
	modeInput       input.Input[os.FileMode]
	userInput       input.Input[string]
	groupInput      input.Input[string]
	sha256Input     input.Input[string]
	forceWriteInput input.Input[bool]
}

// File ensures the target file at Path matches the URL, downloading it when
// it drifts. The node's value reports whether it changed.
func File(urlInput input.Input[string]) *FileBuilder {
	return &FileBuilder{urlInput: urlInput}
}

// Path sets the target destination.
func (b *FileBuilder) Path(pathInput input.Input[string]) *FileBuilder {
	b.pathInput = pathInput
	return b
}

// Mode sets the desired file mode. A nil or zero mode is not checked or applied.
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

// SHA256 sets the expected sha256 (hex) of the content, making the node
// content-idempotent. When unset, only existence and mode are probed (a changed
// upstream file is not detected).
func (b *FileBuilder) SHA256(sha256Input input.Input[string]) *FileBuilder {
	if sha256Input == nil {
		b.sha256Input = nil
		return b
	}

	b.sha256Input = input.Map(sha256Input, func(ctx context.Context, sha256 string) (string, error) {
		return strings.ToLower(sha256), nil
	})

	return b
}

// ForceWrite makes the node download unconditionally when evaluated (no probe),
// so a gate (Filter/IfChanged) can decide when. It cannot be combined with
// SHA256: a forced write does not verify content.
func (b *FileBuilder) ForceWrite(flag ...input.Input[bool]) *FileBuilder {
	b.forceWriteInput = input.Flag(flag...)
	return b
}

// SatisfiedData reports whether the target file matches the expected
// hash (or merely exists when sha256 is empty) and mode, as a data node
// (dry-run safe, memoized once per run). A nil or zero mode is not checked.
func SatisfiedData(targetInput, sha256Input input.Input[string], modeInput input.Input[os.FileMode]) *reconcile.Node[bool] {
	if targetInput == nil {
		panic("httpfile: SatisfiedData: nil target")
	}

	return reconcile.Data("httpfile.satisfied:"+targetInput.Name(), func(ctx context.Context) (bool, error) {
		sha256, err := input.Value(ctx, sha256Input)
		if err != nil {
			return false, err
		}

		var ok bool
		if sha256 != "" {
			ok, err = modules.FileHashMatchesData(
				modules.FileHashData(targetInput),
				input.Static(sha256),
			).Value(ctx)
		} else {
			ok, err = modules.FileExistsData(targetInput).Value(ctx)
		}
		if err != nil || !ok {
			return false, err
		}

		mode, err := input.Value(ctx, modeInput)
		if err != nil {
			return false, err
		}
		if mode != 0 {
			return modules.FileModeMatchesData(
				modules.FileStatData(targetInput),
				input.Static(mode),
			).Value(ctx)
		}

		return true, nil
	}).Node()
}

// Build hands off to the reconcile node builder. It panics if the url or path is
// empty.
func (b *FileBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.urlInput == nil {
		panic("modules: httpfile.File: nil url")
	}
	if b.pathInput == nil {
		panic("modules: httpfile.File: nil path")
	}

	urlInput, targetInput, modeInput, userInput, groupInput := b.urlInput, b.pathInput, b.modeInput, b.userInput, b.groupInput
	sha256Input, forceWriteInput := b.sha256Input, b.forceWriteInput

	name := "httpfile.file:" + targetInput.Name()

	return reconcile.ResourceBool(name,
		func(ctx context.Context) (bool, error) {
			forceWrite, err := input.Value(ctx, forceWriteInput)
			if err != nil {
				return false, err
			}
			if forceWrite {
				sha256, err := input.Value(ctx, sha256Input)
				if err != nil {
					return false, err
				}
				if sha256 != "" {
					return false, fmt.Errorf("modules: httpfile.File: ForceWrite and SHA256 are mutually exclusive")
				}

				return false, nil
			}

			return SatisfiedData(targetInput, sha256Input, modeInput).Value(ctx)
		},
		func(ctx context.Context) error {
			url, err := urlInput.Value(ctx)
			if err != nil {
				return err
			}
			target, err := targetInput.Value(ctx)
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

			ops := gabric.TargetOps(ctx)

			if dir := parentDir(target); dir != "" {
				if err := ops.Mkdir(dir).Recursive().RunErr(); err != nil {
					return err
				}
			}

			tmp := target + ".tmp"

			if err := ops.Exec("curl", "-fsSL", url, "-o", tmp).RunErr(); err != nil {
				return err
			}

			if err := ops.Exec("mv", tmp, target).RunErr(); err != nil {
				return err
			}

			if owner := ownerSpec(user, group); owner != "" {
				if err := ops.Exec("chown", owner, target).RunErr(); err != nil {
					return err
				}
			}

			if mode != 0 {
				if err := ops.Exec("chmod", modeString(mode), target).RunErr(); err != nil {
					return err
				}
			}

			return nil
		})
}
