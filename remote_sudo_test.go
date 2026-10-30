package gabric

import (
	"context"
	"testing"
)

func TestGetRemoteSudo(t *testing.T) {
	if got := GetRemoteSudo(context.Background()); got != nil {
		t.Errorf("GetRemoteSudo() = %+v, want nil", got)
	}
	if HasRemoteSudo(context.Background()) {
		t.Error("HasRemoteSudo() = true without sudo, want false")
	}

	spec := &Sudo{User: "deploy", Flags: []string{"-n"}}
	ctx := NewRemoteSudoContext(context.Background(), spec)

	if !HasRemoteSudo(ctx) {
		t.Error("HasRemoteSudo() = false with sudo, want true")
	}

	got := GetRemoteSudo(ctx)
	if !got.Equal(spec) {
		t.Errorf("GetRemoteSudo() = %+v, want %+v", got, spec)
	}

	got.User = "other"
	got.Flags[0] = "-E"

	if again := GetRemoteSudo(ctx); !again.Equal(spec) {
		t.Errorf("context sudo mutated through getter: %+v", again)
	}

	disabled := NewRemoteSudoContext(ctx, nil)
	if got := GetRemoteSudo(disabled); got != nil {
		t.Errorf("GetRemoteSudo(disabled) = %+v, want nil", got)
	}
}

func TestWithRemoteSudo(t *testing.T) {
	spec := &Sudo{User: "deploy", Flags: []string{"-n"}}

	var got *Sudo

	err := WithRemoteSudo(spec)(context.Background(), func(ctx context.Context) error {
		got = GetRemoteSudo(ctx)
		return nil
	})
	if err != nil {
		t.Fatalf("WithRemoteSudo() error: %v", err)
	}

	if !got.Equal(spec) {
		t.Errorf("sudo = %+v, want %+v", got, spec)
	}
}

func TestWithRemoteSudoNilDisables(t *testing.T) {
	ctx := NewRemoteSudoContext(context.Background(), &Sudo{})

	var got *Sudo

	err := WithRemoteSudo(nil)(ctx, func(ctx context.Context) error {
		got = GetRemoteSudo(ctx)
		return nil
	})
	if err != nil {
		t.Fatalf("WithRemoteSudo(nil) error: %v", err)
	}

	if got != nil {
		t.Errorf("sudo = %+v, want nil", got)
	}
}
