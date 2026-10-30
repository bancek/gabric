package ufw

import "testing"

func TestFieldAfter(t *testing.T) {
	s := "Package: ufw\n  Candidate: 1.2.3\n"

	if got := fieldAfter(s, "Candidate:"); got != "1.2.3" {
		t.Errorf("fieldAfter() = %q, want %q", got, "1.2.3")
	}

	if got := fieldAfter(s, "Missing:"); got != "" {
		t.Errorf("fieldAfter(missing) = %q, want %q", got, "")
	}
}
