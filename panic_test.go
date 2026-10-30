package gabric

import (
	"errors"
	"testing"
)

func TestPanicError(t *testing.T) {
	sentinel := errors.New("boom")

	if err := PanicError(sentinel); !errors.Is(err, sentinel) {
		t.Errorf("PanicError(error) = %v, want %v", err, sentinel)
	}

	err := PanicError("boom")
	if err != nil {
		if err.Error() != "boom" {
			t.Errorf("PanicError(string) = %v, want %q", err, "boom")
		}
	} else {
		t.Errorf("PanicError(string) = nil, want %q", "boom")
	}
}
