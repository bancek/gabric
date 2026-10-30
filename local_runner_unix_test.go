//go:build unix

package gabric

import (
	"errors"
	"io"
	"testing"
)

func TestNewLocalRunnerExitError(t *testing.T) {
	runner := NewLocalRunner(func(cmd string) []string {
		return []string{"/bin/sh", "-c", cmd}
	})

	err := runner(testContext(t), "exit 3", nil, io.Discard, io.Discard)

	exitErr, ok := errors.AsType[*ExecExitError](err)
	if !ok {
		t.Fatalf("runner() error = %T, want *ExecExitError", err)
	}
	if exitErr.Status != 3 {
		t.Errorf("Status = %d, want 3", exitErr.Status)
	}
}
