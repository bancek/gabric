package ufw

import "testing"

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic", name)
		}
	}()

	fn()
}
