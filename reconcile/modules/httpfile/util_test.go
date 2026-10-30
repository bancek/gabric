package httpfile

import (
	"os"
	"testing"
)

func TestModeString(t *testing.T) {
	tests := []struct {
		mode os.FileMode
		want string
	}{
		{0o644, "644"},
		{0o600, "600"},
		{0o755, "755"},
	}

	for _, tt := range tests {
		if got := modeString(tt.mode); got != tt.want {
			t.Errorf("modeString(%v) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
