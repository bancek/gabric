package modules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func contentHash(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])
}

// FileContentData reads the target file at path as a data node (dry-run safe,
// memoized once per run), for example a version marker. It returns "" when the
// file does not exist; any other read error propagates.
func FileContentData(pathInput input.Input[string]) *reconcile.Node[string] {
	if pathInput == nil {
		panic("modules: FileContentData: nil path")
	}

	return reconcile.Data("modules.file-content-data:"+pathInput.Name(), func(ctx context.Context) (string, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return "", err
		}

		content, err := gabric.TargetOps(ctx).Get(path).StringErr()
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}

		return content, err
	}).Node()
}

// PathExistsData reports whether the target path exists at all, as a data node
// (dry-run safe, memoized once per run). Use FileExistsData for the regular-file
// check.
func PathExistsData(pathInput input.Input[string]) *reconcile.Node[bool] {
	if pathInput == nil {
		panic("modules: PathExistsData: nil path")
	}

	return reconcile.Data("modules.path-exists-data:"+pathInput.Name(), func(ctx context.Context) (bool, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return false, err
		}

		return gabric.TargetOps(ctx).Exists(path).RunErr()
	}).Node()
}

// FileExistsData reports whether the target file at path exists, as a data node
// (dry-run safe, memoized once per run).
func FileExistsData(pathInput input.Input[string]) *reconcile.Node[bool] {
	if pathInput == nil {
		panic("modules: FileExistsData: nil path")
	}

	return reconcile.Data("modules.file-exists-data:"+pathInput.Name(), func(ctx context.Context) (bool, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return false, err
		}

		return gabric.TargetOps(ctx).Exists(path).File().RunErr()
	}).Node()
}

// DirExistsData reports whether the target path is a directory, as a data node
// (dry-run safe, memoized once per run).
func DirExistsData(pathInput input.Input[string]) *reconcile.Node[bool] {
	if pathInput == nil {
		panic("modules: DirExistsData: nil path")
	}

	return reconcile.Data("modules.dir-exists-data:"+pathInput.Name(), func(ctx context.Context) (bool, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return false, err
		}

		return gabric.TargetOps(ctx).Exists(path).Dir().RunErr()
	}).Node()
}

// FileHashData reads the sha256 of the target file at path as a data node
// (dry-run safe, memoized once per run). It returns "" when the file does not
// exist; any other read error propagates.
func FileHashData(pathInput input.Input[string]) *reconcile.Node[string] {
	if pathInput == nil {
		panic("modules: FileHashData: nil path")
	}

	return reconcile.Data("modules.file-hash-data:"+pathInput.Name(), func(ctx context.Context) (string, error) {
		path, err := pathInput.Value(ctx)
		if err != nil {
			return "", err
		}

		ops := gabric.TargetOps(ctx)

		if local, ok := ops.(*gabric.LocalOps); ok {
			return localFileHash(local, path)
		}

		out, err := CommandOutputData(input.List("sha256sum", path)).Value(ctx)
		if IsExitStatus(err, 127) {
			out, err = CommandOutputData(input.List("shasum", "-a", "256", path)).Value(ctx)
		}
		if err != nil {
			if !IsExitStatus(err, 1) {
				return "", err
			}

			exists, existsErr := PathExistsData(input.Static(path)).Value(ctx)
			if existsErr != nil {
				return "", existsErr
			}
			if !exists {
				return "", nil
			}

			return "", err
		}

		fields := strings.Fields(out)
		if len(fields) == 0 {
			return "", fmt.Errorf("file hash %s: empty output", path)
		}

		return fields[0], nil
	}).Node()
}

// localFileHash streams a local file through SHA-256. LocalTarget does not need
// the target's sha256sum command, and reading the file locally does not cross a
// network boundary.
func localFileHash(ops *gabric.LocalOps, path string) (string, error) {
	h := sha256.New()

	err := ops.Get(path).WithReaderErr(func(r io.Reader) error {
		_, err := io.Copy(h, r)
		return err
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileHashMatchesData reports whether hash equals want. A missing file (an empty
// hash) never matches, including an empty want.
func FileHashMatchesData(hash *reconcile.Node[string], wantInput input.Input[string]) *reconcile.Node[bool] {
	if wantInput == nil {
		panic("modules: FileHashMatchesData: nil want")
	}

	return reconcile.Map(hash, func(ctx context.Context, got string) (bool, error) {
		want, err := wantInput.Value(ctx)
		if err != nil {
			return false, err
		}

		return got != "" && got == want, nil
	})
}

// FileContentMatchesData reports whether the target file's sha256 equals the
// sha256 of content. A missing file never matches.
func FileContentMatchesData(hash *reconcile.Node[string], contentInput input.Input[[]byte]) *reconcile.Node[bool] {
	if contentInput == nil {
		panic("modules: FileContentMatchesData: nil content")
	}

	return reconcile.Map(hash, func(ctx context.Context, got string) (bool, error) {
		content, err := contentInput.Value(ctx)
		if err != nil {
			return false, err
		}

		return got != "" && got == contentHash(content), nil
	})
}
