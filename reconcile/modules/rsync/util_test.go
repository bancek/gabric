package rsync

import "testing"

func TestSyncHasChanges(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"metadata only", ".f...p..... file.txt\n", false},
		{"transfer", ">f+++++++++ new.txt\n", true},
		{"deleted", "*deleting   old.txt\n", true},
		{"created dir", "cd+++++++++ dir/\n", true},
		{"mixed", ".f...p..... a\n>f+++++++++ b\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rsyncHasChanges(tt.in); got != tt.want {
				t.Errorf("rsyncHasChanges(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
