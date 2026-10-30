package gabrice2e

import (
	"fmt"
	"os"
	"testing"
)

// Main gates an e2e test package on GABRIC_E2E.
func Main(m *testing.M) {
	if os.Getenv("GABRIC_E2E") == "" {
		fmt.Fprintln(os.Stderr, "skipping e2e tests: set GABRIC_E2E=1")
		os.Exit(0)
	}

	os.Exit(m.Run())
}
