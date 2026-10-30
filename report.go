package gabric

import (
	"context"
	"strings"

	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

// WithReport is middleware that collects reconcile reports from every run
// started by next, including isolated runs, and logs the combined report when
// next finishes. It does not open a run. If ctx already carries a collector,
// WithReport logs that one; otherwise it installs a new collector. Use it alone
// with isolated Run/RunNode calls, or combine it with WithReconcile for a shared
// run:
//
//	var _ = gabric.Use(gabric.WithReport)
//	var _ = gabric.Use(gabric.WithReport, gabric.WithReconcile)
//
// Each report line is logged as its own record (prefixed "reconcile: "), so a
// multi-line report stays readable in the text log instead of being escaped
// into a single message.
func WithReport(ctx context.Context, next middleware.Next) error {
	collector := reconcile.GetReportCollector(ctx)
	if collector == nil {
		collector = &reconcile.ReportCollector{}
		ctx = reconcile.WithReportCollector(ctx, collector)
	}

	defer func() {
		LogReport(ctx, collector.Results())
	}()

	return next(ctx)
}

// LogReport logs reconcile results using Logger(ctx), one record per line with
// the "reconcile: " prefix. It is the logging used by WithReport; call it
// directly to log a collector's Results or an isolated report.
func LogReport(ctx context.Context, results []reconcile.ReportResult) {
	if len(results) == 0 {
		return
	}

	log := Logger(ctx)

	for line := range strings.SplitSeq(reconcile.FormatReport(results), "\n") {
		log.Info("reconcile: " + line)
	}
}

// WithReconcileReport runs next inside a single reconcile.Run and logs its
// report when the run finishes. It is shorthand for WithReport around
// WithReconcile:
//
//	var _ = gabric.Use(gabric.WithReconcileReport)
func WithReconcileReport(ctx context.Context, next middleware.Next) error {
	return WithReport(ctx, func(ctx context.Context) error {
		return WithReconcile(ctx, next)
	})
}
