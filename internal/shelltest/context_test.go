//go:build unix

package shelltest

import (
	"context"
	"slices"
	"testing"
)

func TestSudoContext(t *testing.T) {
	parent := context.Background()

	child := withSudo(parent, &Sudo{User: "app", Flags: []string{"-n"}})

	if got := sudoFromContext(parent); got != nil {
		t.Errorf("parent sudo = %v, want nil", got)
	}

	got := sudoFromContext(child)
	if got == nil {
		t.Fatal("child sudo = nil, want spec")
	}
	if got.User != "app" {
		t.Errorf("User = %q, want %q", got.User, "app")
	}
	if len(got.Flags) != 1 || got.Flags[0] != "-n" {
		t.Errorf("Flags = %v, want [-n]", got.Flags)
	}
}

func TestSudoContextOverrides(t *testing.T) {
	ctx := withSudo(context.Background(), &Sudo{})
	ctx = withSudo(ctx, &Sudo{User: "app"})

	got := sudoFromContext(ctx)
	if got == nil || got.User != "app" {
		t.Errorf("sudo = %v, want user app", got)
	}
}

func TestEnvContext(t *testing.T) {
	parent := context.Background()

	if got := envFromContext(parent); got != nil {
		t.Errorf("parent env = %v, want nil", got)
	}

	ctx := withEnv(parent, []string{"A=1"})
	ctx = withSudo(ctx, &Sudo{User: "app"})

	if got, want := envFromContext(ctx), []string{"A=1"}; !slices.Equal(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
	if got := sudoFromContext(ctx); got == nil || got.User != "app" {
		t.Errorf("sudo = %v, want user app", got)
	}

	ctx = withEnv(ctx, []string{"B=2"})
	if got, want := envFromContext(ctx), []string{"B=2"}; !slices.Equal(got, want) {
		t.Errorf("env after override = %v, want %v", got, want)
	}
}
