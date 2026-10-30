package gabric

import (
	"context"
)

// Remote returns remote operations bound to ctx.
func Remote(ctx context.Context) *RemoteOps {
	return &RemoteOps{ctx: ctx}
}

// RemoteOps provides remote command and file operations bound to a context.
type RemoteOps struct {
	ctx context.Context
}
