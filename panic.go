package gabric

import "fmt"

// PanicError converts a recovered panic value into an error: an error value is
// returned unchanged; anything else is wrapped. It lets panic-style and
// error-first code share error semantics, so errors.Is/errors.As work the same
// for a task that panics and one that returns.
func PanicError(r any) error {
	if err, ok := r.(error); ok {
		return err
	}

	return fmt.Errorf("%v", r)
}
