package reconcile

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ReportStatus is the outcome of evaluating one resource node.
type ReportStatus int

const (
	// ReportOK is a resource whose probe found it already satisfied.
	ReportOK ReportStatus = iota
	// ReportChanged is a resource whose apply ran and succeeded.
	ReportChanged
	// ReportWouldChange is a resource that drifted in a dry run (apply skipped).
	ReportWouldChange
	// ReportFailed is a resource whose evaluation returned an error.
	ReportFailed
)

func (s ReportStatus) String() string {
	switch s {
	case ReportOK:
		return "ok"
	case ReportChanged:
		return "changed"
	case ReportWouldChange:
		return "would-change"
	case ReportFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ReportResult is the recorded outcome of one resource node.
type ReportResult struct {
	Name   string
	Status ReportStatus
	Err    error // set only for ReportFailed
}

// ReportCollector aggregates report results across one or more runs. Its zero
// value is ready to use and its methods are safe for concurrent use.
type ReportCollector struct {
	mu      sync.Mutex
	results []ReportResult
}

// Collect appends results to the collector.
func (c *ReportCollector) Collect(results []ReportResult) {
	if len(results) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.results = append(c.results, results...)
}

// Results returns a copy of the collected results in completion order.
func (c *ReportCollector) Results() []ReportResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.results)
}

const reportCollectorKey contextKey[*ReportCollector] = "reconcile.reportCollector"

// WithReportCollector returns a context carrying c, so every Run started from
// it appends its results to the collector.
func WithReportCollector(ctx context.Context, c *ReportCollector) context.Context {
	return withContextValue(ctx, reportCollectorKey, c)
}

// GetReportCollector returns the collector carried by ctx, or nil when none is
// set.
func GetReportCollector(ctx context.Context) *ReportCollector {
	collector, _ := contextValue(ctx, reportCollectorKey)
	return collector
}

// HasReportCollector reports whether ctx carries a report collector.
func HasReportCollector(ctx context.Context) bool {
	collector, ok := contextValue(ctx, reportCollectorKey)
	return ok && collector != nil
}

// collectReport appends the current run's results to the context's collector,
// if there is one.
func collectReport(ctx context.Context) {
	c := GetReportCollector(ctx)
	if c == nil {
		return
	}

	c.Collect(Report(ctx))
}

// Report returns the resource results of the active run, in first-record order.
// It returns nil outside a Run. To aggregate results across runs, install a
// ReportCollector with WithReportCollector; Run appends to it when it finishes.
// Because evaluation aborts on error, the failed entry is normally the last one
// in a sequential run; a run evaluated concurrently (for example gabric.Parallel)
// may record several.
func Report(ctx context.Context) []ReportResult {
	rs, ok := runFrom(ctx)
	if !ok {
		return nil
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	out := make([]ReportResult, 0, len(rs.resultOrder))

	for _, id := range rs.resultOrder {
		out = append(out, *rs.results[id])
	}

	return out
}

// FormatReport renders one line per result ("  <status> <name> (err)") and a
// summary ("N ok, N changed, N would-change, N failed").
func FormatReport(results []ReportResult) string {
	var b strings.Builder

	var ok, changed, wouldChange, failed int

	for _, r := range results {
		fmt.Fprintf(&b, "  %-12s %s", r.Status, r.Name)

		if r.Err != nil {
			fmt.Fprintf(&b, " (%v)", r.Err)
		}

		b.WriteByte('\n')

		switch r.Status {
		case ReportOK:
			ok++
		case ReportChanged:
			changed++
		case ReportWouldChange:
			wouldChange++
		case ReportFailed:
			failed++
		}
	}

	fmt.Fprintf(&b, "%d ok, %d changed, %d would-change, %d failed", ok, changed, wouldChange, failed)

	return b.String()
}

// reportResult records a resource node's outcome in the active run's report.
// It is a no-op outside a run (a node's fn only runs inside one).
func reportResult(ctx context.Context, id *node, status ReportStatus, err error) {
	if rs, ok := runFrom(ctx); ok {
		rs.recordResult(id, status, err)
	}
}

// reportCurrent records a resource outcome for the node currently evaluating.
// It uses the evaluation frame rather than a captured identity, so a cloned or
// middleware-wrapped resource reports under its own identity.
func reportCurrent(ctx context.Context, status ReportStatus, err error) {
	if frame := evalFrameFrom(ctx); frame != nil {
		reportResult(ctx, frame.node, status, err)
	}
}
