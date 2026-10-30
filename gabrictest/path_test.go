package gabrictest

import (
	"runtime"
	"testing"
)

func TestVirtualPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"absolute", "/srv/app", "/srv/app"},
		{"relative", "srv/app", "/srv/app"},
		{"clean", "/srv/../app", "/app"},
		{"empty", "", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := virtualPath(tt.in); got != tt.want {
				t.Errorf("virtualPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestVirtualPathWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path semantics")
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"backslashes", `\srv\app`, "/srv/app"},
		{"drive", `C:\srv\app`, "/srv/app"},
		{"drive root", `C:`, "/"},
		{"drive clean", `C:\srv\..\app`, "/app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := virtualPath(tt.in); got != tt.want {
				t.Errorf("virtualPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestVirtualPathKeepsUnixColon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("colon starts a Windows volume")
	}

	if got, want := virtualPath("c:file"), "/c:file"; got != want {
		t.Errorf("virtualPath(%q) = %q, want %q", "c:file", got, want)
	}
}
