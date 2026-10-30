package gabric

import (
	"context"
	"testing"
)

func TestGetCurrentTaskArg(t *testing.T) {
	if arg := GetCurrentTaskArg(context.Background()); arg != "" {
		t.Errorf("GetCurrentTaskArg() = %q, want empty", arg)
	}
	if HasCurrentTaskArg(context.Background()) {
		t.Error("HasCurrentTaskArg() = true without an arg, want false")
	}

	ctx := NewCurrentTaskArgContext(context.Background(), "staging")

	if arg := GetCurrentTaskArg(ctx); arg != "staging" {
		t.Errorf("GetCurrentTaskArg() = %q, want %q", arg, "staging")
	}
	if !HasCurrentTaskArg(ctx) {
		t.Error("HasCurrentTaskArg() = false with an arg, want true")
	}
}
