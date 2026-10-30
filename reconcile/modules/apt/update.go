package apt

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// apt touches the lists partial directory on every refresh, including a no-op
// update. Prefer apt-daily's success stamp when it is fresh.
var aptCachePaths = []string{
	"/var/lib/apt/periodic/update-success-stamp",
	"/var/lib/apt/lists/partial",
}

// UpdateBuilder builds a node that refreshes the apt package index.
type UpdateBuilder struct {
	maxAgeInput input.Input[time.Duration]
}

// Update refreshes the apt package index. Without MaxAge it always runs when
// evaluated.
func Update() *UpdateBuilder {
	return &UpdateBuilder{}
}

// MaxAge refreshes the index only when the apt cache is older than maxAge. A
// nil or non-positive duration keeps the default behavior and always refreshes.
func (b *UpdateBuilder) MaxAge(maxAgeInput input.Input[time.Duration]) *UpdateBuilder {
	next := *b
	next.maxAgeInput = maxAgeInput
	return &next
}

// Build hands off to the reconcile node builder.
func (b *UpdateBuilder) Build() *reconcile.ResourceBuilder[bool] {
	maxAgeInput := b.maxAgeInput

	var check func(context.Context) (bool, error)
	if maxAgeInput != nil {
		fresh := cacheFreshData(aptCachePaths, maxAgeInput)
		check = func(ctx context.Context) (bool, error) {
			return fresh.Value(ctx)
		}
	}

	return reconcile.ResourceBool("apt.update", check, func(ctx context.Context) error {
		return gabric.TargetOps(ctx).Exec("apt-get", "update").RunErr()
	})
}

func cacheFreshData(paths []string, maxAgeInput input.Input[time.Duration]) *reconcile.Node[bool] {
	return reconcile.Data("apt.cache-fresh", func(ctx context.Context) (bool, error) {
		maxAge, err := maxAgeInput.Value(ctx)
		if err != nil {
			return false, err
		}
		if maxAge <= 0 {
			return false, nil
		}

		for _, path := range paths {
			out, err := modules.CommandOutputData(input.List("stat", "-c", "%Y", path)).Value(ctx)
			if err != nil {
				if modules.IsExitStatus(err, 1) {
					continue
				}

				return false, err
			}

			sec, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
			if err != nil {
				return false, fmt.Errorf("apt: parse cache mtime for %s: %w", path, err)
			}
			if time.Since(time.Unix(sec, 0)) <= maxAge {
				return true, nil
			}
		}

		return false, nil
	}).Node()
}
