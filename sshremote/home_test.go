package sshremote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir() error: %v", err)
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "tilde", in: "~", want: home},
		{name: "tilde path", in: "~/.ssh/id_ed25519", want: filepath.Join(home, ".ssh", "id_ed25519")},
		{name: "absolute", in: "/etc/ssh/ssh_config", want: "/etc/ssh/ssh_config"},
		{name: "relative", in: "relative/path", want: "relative/path"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandHome(tt.in)
			if err != nil {
				t.Fatalf("expandHome(%q) error: %v", tt.in, err)
			}

			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
