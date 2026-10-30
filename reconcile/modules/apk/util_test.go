package apk

import "testing"

func TestVersionFrom(t *testing.T) {
	tests := []struct {
		name string
		out  string
		pkg  string
		want string
	}{
		{"installed", "curl-8.14.1-r2 aarch64 {curl} (curl) [installed]\n", "curl", "8.14.1-r2"},
		{"candidate", "curl-8.14.1-r2\n", "curl", "8.14.1-r2"},
		{"hyphenated", "ca-certificates-20261020-r0 aarch64 [installed]\n", "ca-certificates", "20261020-r0"},
		{"missing", "", "curl", ""},
		{"other", "wget-1.24.4-r0 aarch64 [installed]\n", "curl", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionFrom(tt.out, tt.pkg); got != tt.want {
				t.Errorf("versionFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}
