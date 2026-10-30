package rsync

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// SyncBuilder builds a node that copies and updates a local tree at a remote
// path (upload only).
type SyncBuilder struct {
	localInput    input.Input[string]
	remoteInput   input.Input[string]
	deleteInput   input.Input[bool]
	excludesInput input.Input[[]string]
}

// Sync ensures the local tree is copied to the remote path. The node value
// reports whether it changed; notify via Build().Notify(...).
func Sync(localInput input.Input[string]) *SyncBuilder {
	return &SyncBuilder{localInput: localInput}
}

// Remote sets the remote path.
func (b *SyncBuilder) Remote(remoteInput input.Input[string]) *SyncBuilder {
	b.remoteInput = remoteInput
	return b
}

// Delete removes remote files that no longer exist locally.
func (b *SyncBuilder) Delete(flag ...input.Input[bool]) *SyncBuilder {
	b.deleteInput = input.Flag(flag...)
	return b
}

// Exclude adds rsync --exclude patterns.
func (b *SyncBuilder) Exclude(excludesInput input.Input[[]string]) *SyncBuilder {
	b.excludesInput = excludesInput
	return b
}

func rsyncOps(ctx context.Context, local, remote string, del bool, excludes []string) (*gabric.RsyncOps, error) {
	o, err := gabric.TargetRsyncOps(ctx)
	if err != nil {
		return nil, err
	}

	o = o.Local(local).Remote(remote).Upload()

	if del {
		o.Delete()
	}

	if len(excludes) > 0 {
		o.Exclude(excludes...)
	}

	return o, nil
}

// SatisfiedData reports whether the local tree is already copied to the
// remote path, as a data node (dry-run safe, memoized once per run). It runs the
// same dry-run itemize as Sync.
func SatisfiedData(localInput, remoteInput input.Input[string], deleteInput input.Input[bool], excludesInput input.Input[[]string]) *reconcile.Node[bool] {
	if localInput == nil {
		panic("rsync: SatisfiedData: nil local")
	}
	if remoteInput == nil {
		panic("rsync: SatisfiedData: nil remote")
	}

	return reconcile.Data("rsync.satisfied:"+localInput.Name()+"->"+remoteInput.Name(), func(ctx context.Context) (bool, error) {
		local, remote, delete, excludes, err := resolve(ctx, localInput, remoteInput, deleteInput, excludesInput)
		if err != nil {
			return false, err
		}

		dry, err := rsyncOps(ctx, local, remote, delete, excludes)
		if err != nil {
			return false, err
		}

		dry.AddFlags("-n", "-i").CaptureStdout()

		if err := dry.RunErr(); err != nil {
			return false, err
		}

		return !rsyncHasChanges(dry.StdoutStr()), nil
	}).Node()
}

// Build hands off to the reconcile node builder.
func (b *SyncBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.localInput == nil {
		panic("rsync: Sync: nil local")
	}
	if b.remoteInput == nil {
		panic("rsync: Sync: nil remote")
	}

	localInput, remoteInput, deleteInput, excludesInput := b.localInput, b.remoteInput, b.deleteInput, b.excludesInput

	return reconcile.ResourceBool("rsync.sync:"+localInput.Name()+"->"+remoteInput.Name(),
		func(ctx context.Context) (bool, error) {
			return SatisfiedData(localInput, remoteInput, deleteInput, excludesInput).Value(ctx)
		},
		func(ctx context.Context) error {
			local, remote, delete, excludes, err := resolve(ctx, localInput, remoteInput, deleteInput, excludesInput)
			if err != nil {
				return err
			}

			o, err := rsyncOps(ctx, local, remote, delete, excludes)
			if err != nil {
				return err
			}

			return o.RunErr()
		})
}

func resolve(ctx context.Context, localInput, remoteInput input.Input[string], deleteInput input.Input[bool], excludesInput input.Input[[]string]) (string, string, bool, []string, error) {
	local, err := localInput.Value(ctx)
	if err != nil {
		return "", "", false, nil, err
	}
	remote, err := remoteInput.Value(ctx)
	if err != nil {
		return "", "", false, nil, err
	}
	delete := false
	if deleteInput != nil {
		delete, err = deleteInput.Value(ctx)
		if err != nil {
			return "", "", false, nil, err
		}
	}
	var excludes []string
	if excludesInput != nil {
		excludes, err = excludesInput.Value(ctx)
		if err != nil {
			return "", "", false, nil, err
		}
	}

	return local, remote, delete, excludes, nil
}
