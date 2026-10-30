package gabric

import (
	"context"
	"testing"
)

func TestGetRemoteWorkingDir(t *testing.T) {
	if dir := GetRemoteWorkingDir(context.Background()); dir != "" {
		t.Errorf("GetRemoteWorkingDir() = %q, want empty", dir)
	}
	if HasRemoteWorkingDir(context.Background()) {
		t.Error("HasRemoteWorkingDir() = true without a working dir, want false")
	}

	ctx := NewRemoteWorkingDirContext(context.Background(), "/srv/app")

	if dir := GetRemoteWorkingDir(ctx); dir != "/srv/app" {
		t.Errorf("GetRemoteWorkingDir() = %q, want %q", dir, "/srv/app")
	}
	if !HasRemoteWorkingDir(ctx) {
		t.Error("HasRemoteWorkingDir() = false with a working dir, want true")
	}
}

func TestResolveRemotePath(t *testing.T) {
	base := context.Background()
	withWorkingDir := NewRemoteWorkingDirContext(base, "/srv/app")

	tests := []struct {
		name string
		ctx  context.Context
		in   string
		want string
	}{
		{name: "no working dir relative", ctx: base, in: "config", want: "config"},
		{name: "no working dir absolute", ctx: base, in: "/etc/config", want: "/etc/config"},
		{name: "relative", ctx: withWorkingDir, in: "config", want: "/srv/app/config"},
		{name: "absolute", ctx: withWorkingDir, in: "/etc/config", want: "/etc/config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveRemotePath(tt.ctx, tt.in); got != tt.want {
				t.Errorf("ResolveRemotePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
