package docker

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// ComposeBuilder builds a node that brings a docker compose project up.
type ComposeBuilder struct {
	fileInput        input.Input[string]
	buildImagesInput input.Input[bool]
}

// Compose ensures the compose project defined by file (for example
// /srv/www/compose.yaml) is up. The probe runs `docker compose --dry-run up` and
// treats any action Compose would take (create, recreate, start, ...) as drift;
// the apply runs the real `up`. Change detection for build inputs is delegated
// to Compose's config hash, so a template can put external inputs (for example a
// git revision) in a service label to trigger a recreate. The node's value
// reports whether it changed.
func Compose(fileInput input.Input[string]) *ComposeBuilder {
	return &ComposeBuilder{fileInput: fileInput}
}

// BuildImages also builds images before starting (up --build).
func (b *ComposeBuilder) BuildImages(flag ...input.Input[bool]) *ComposeBuilder {
	b.buildImagesInput = input.Flag(flag...)
	return b
}

// ComposeSatisfiedData reports whether the compose project defined by file is
// converged, as a data node (dry-run safe, memoized once per run). It runs the
// same dry-run probe as Compose.
func ComposeSatisfiedData(fileInput input.Input[string]) *reconcile.Node[bool] {
	if fileInput == nil {
		panic("docker: ComposeSatisfiedData: nil file")
	}

	return reconcile.Data("docker.compose-satisfied:"+fileInput.Name(), func(ctx context.Context) (bool, error) {
		file, err := fileInput.Value(ctx)
		if err != nil {
			return false, err
		}

		exists, err := modules.FileExistsData(input.Static(file)).Value(ctx)
		if err != nil {
			return false, err
		}

		// The compose file is not deployed yet: drift, without probing.
		if !exists {
			return false, nil
		}

		out, err := modules.CommandStderrData(input.List("docker", "compose", "-f", file, "--progress", "json", "--dry-run", "up", "-d", "--remove-orphans")).Value(ctx)
		if err != nil {
			return false, err
		}

		return !composeWouldChange(out), nil
	}).Node()
}

// Build hands off to the reconcile node builder. It panics if the compose file
// path is nil.
func (b *ComposeBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.fileInput == nil {
		panic("modules: Compose: nil file")
	}

	fileInput, buildImagesInput := b.fileInput, b.buildImagesInput

	return reconcile.ResourceBool("docker.compose:"+fileInput.Name(),
		func(ctx context.Context) (bool, error) {
			return ComposeSatisfiedData(fileInput).Value(ctx)
		},
		func(ctx context.Context) error {
			file, err := fileInput.Value(ctx)
			if err != nil {
				return err
			}
			buildImages, err := input.Value(ctx, buildImagesInput)
			if err != nil {
				return err
			}

			apply := []string{"docker", "compose", "-f", file, "up", "-d"}
			if buildImages {
				apply = append(apply, "--build")
			}
			apply = append(apply, "--remove-orphans")

			return gabric.TargetOps(ctx).Exec(apply...).RunErr()
		})
}

// composeWouldChange parses the NDJSON `docker compose --progress json --dry-run
// up` writes to stderr: a record whose text is not "Running" means Compose would
// act. Non-JSON lines (warnings) are ignored.
func composeWouldChange(out string) bool {
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var record struct {
			Text   string `json:"text"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		action := record.Text
		if action == "" {
			action = record.Status
		}
		if action == "" {
			continue
		}
		if action != "Running" {
			return true
		}
	}

	return false
}
