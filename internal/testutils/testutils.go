// Package testutils provides gomega-like Eventually/Consistently helpers shared
// by gabric's tests.
package testutils

import "time"

// pollInterval is the delay between condition checks.
const pollInterval = 5 * time.Millisecond

// TB is the subset of testing.TB the helpers need.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Eventually retries cond until it returns true or timeout elapses.
func Eventually(tb TB, timeout time.Duration, cond func() bool) {
	tb.Helper()

	deadline := time.Now().Add(timeout)

	for {
		if cond() {
			return
		}

		if !time.Now().Before(deadline) {
			tb.Fatalf("condition not met within %s", timeout)
		}

		time.Sleep(pollInterval)
	}
}

// Consistently asserts cond keeps returning true for duration.
func Consistently(tb TB, duration time.Duration, cond func() bool) {
	tb.Helper()

	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		if !cond() {
			tb.Fatalf("condition did not hold for the whole %s", duration)
		}

		time.Sleep(pollInterval)
	}
}
