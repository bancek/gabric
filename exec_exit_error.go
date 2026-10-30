package gabric

import "fmt"

// ExecExitError reports that a command exited with a non-zero status. Status is
// the command exit status and Err is the underlying execution error.
type ExecExitError struct {
	Status int
	Err    error
}

// Error returns the command exit status.
func (e *ExecExitError) Error() string {
	return fmt.Sprintf("command exited with status %d", e.Status)
}

// Unwrap returns the underlying execution error.
func (e *ExecExitError) Unwrap() error {
	return e.Err
}

// ExitCode returns the command exit status.
func (e *ExecExitError) ExitCode() int {
	return e.Status
}
