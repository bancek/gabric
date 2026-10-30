package apk

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

const apkIndexStat = "stat -c %Y /var/cache/apk/APKINDEX.*"

// UpdateBuilder builds a node that refreshes the apk package index.
type UpdateBuilder struct {
	maxAgeInput input.Input[time.Duration]
}

// Update refreshes the apk package index. Without MaxAge it always runs when
// evaluated.
func Update() *UpdateBuilder {
	return &UpdateBuilder{}
}

// MaxAge refreshes the index only when the apk cache is older than maxAge. A
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
		fresh := cacheFreshData(maxAgeInput)
		check = func(ctx context.Context) (bool, error) {
			return fresh.Value(ctx)
		}
	}

	return reconcile.ResourceBool("apk.update", check, func(ctx context.Context) error {
		return gabric.TargetOps(ctx).Exec("apk", "update").RunErr()
	})
}

func cacheFreshData(maxAgeInput input.Input[time.Duration]) *reconcile.Node[bool] {
	return reconcile.Data("apk.cache-fresh", func(ctx context.Context) (bool, error) {
		maxAge, err := maxAgeInput.Value(ctx)
		if err != nil {
			return false, err
		}
		if maxAge <= 0 {
			return false, nil
		}

		out, err := modules.CommandOutputData(input.List("sh", "-c", apkIndexStat)).Value(ctx)
		if err != nil {
			if modules.IsExitStatus(err, 1) {
				return false, nil
			}

			return false, err
		}

		fresh := false

		for line := range strings.SplitSeq(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			sec, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				return false, fmt.Errorf("apk: parse cache mtime %q: %w", line, err)
			}

			fresh = true

			if time.Since(time.Unix(sec, 0)) > maxAge {
				return false, nil
			}
		}

		return fresh, nil
	}).Node()
}
