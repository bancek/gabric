package gabric

import (
	"context"
)

// Local returns local operations bound to ctx.
func Local(ctx context.Context) *LocalOps {
	return &LocalOps{ctx: ctx}
}

// LocalOps provides local command and file operations bound to a context.
type LocalOps struct {
	ctx context.Context
}
