package dryrun

import (
	"context"
	"testing"
)

func TestEnabled(t *testing.T) {
	ctx := context.Background()

	if Enabled(ctx) {
		t.Fatal("Enabled() = true without With")
	}

	ctx = With(ctx)
	if !Enabled(ctx) {
		t.Fatal("Enabled() = false with With")
	}

	derived := context.WithValue(ctx, struct{}{}, "value")
	if !Enabled(derived) {
		t.Fatal("Enabled() = false on a derived context")
	}
}
