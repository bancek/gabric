package gabric

import (
	"os"
	"testing"
)

func TestOwnerSpec(t *testing.T) {
	tests := []struct {
		user, group, want string
	}{
		{"", "", ""},
		{"app", "", "app"},
		{"", "app", ":app"},
		{"app", "app", "app:app"},
	}

	for _, tt := range tests {
		if got := ownerSpec(tt.user, tt.group); got != tt.want {
			t.Errorf("ownerSpec(%q, %q) = %q, want %q", tt.user, tt.group, got, tt.want)
		}
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode os.FileMode
		want string
	}{
		{0o600, "600"},
		{0o644, "644"},
		{0o755, "755"},
	}

	for _, tt := range tests {
		if got := modeString(tt.mode); got != tt.want {
			t.Errorf("modeString(%v) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
