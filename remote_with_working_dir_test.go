//go:build unix

package gabric

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRemoteWithWorkingDirErr(t *testing.T) {
	ctx := testContext(t)

	sentinel := errors.New("boom")

	err := Remote(ctx).WithWorkingDirErr("/srv/app", func(ctx context.Context) error {
		if dir := GetRemoteWorkingDir(ctx); dir != "/srv/app" {
			t.Errorf("remote working dir = %q, want %q", dir, "/srv/app")
		}

		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("WithWorkingDirErr() error = %v, want %v", err, sentinel)
	}
}

func TestRemoteWithWorkingDir(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	if err := core.FS().MkdirAll("/srv/app/sub", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	tests := []struct {
		name string
		base context.Context
		dir  string
		want string
	}{
		{
			name: "absolute",
			base: ctx,
			dir:  "/srv/app/sub",
			want: "/srv/app/sub",
		},
		{
			name: "relative",
			base: NewRemoteWorkingDirContext(ctx, "/srv/app"),
			dir:  "sub",
			want: "/srv/app/sub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Remote(tt.base).WithWorkingDir(tt.dir, func(ctx context.Context) {
				got := strings.TrimSpace(Remote(ctx).Exec("pwd").CaptureStdout().Run().StdoutStr())
				if got != tt.want {
					t.Errorf("pwd = %q, want %q", got, tt.want)
				}
			})
		})
	}
}
