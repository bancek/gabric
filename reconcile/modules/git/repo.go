package git

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// RemoteRevisionData reads the remote HEAD revision of repo as a data node (memoized per
// run, so several probes share one git ls-remote). It runs in a dry run.
func RemoteRevisionData(repoInput input.Input[string]) *reconcile.Node[string] {
	if repoInput == nil {
		panic("git: RemoteRevisionData: nil repo")
	}

	return reconcile.Data("git.remote-revision:"+repoInput.Name(), func(ctx context.Context) (string, error) {
		repo, err := repoInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("git", "ls-remote", repo, "HEAD")).Value(ctx)
		if err != nil {
			return "", err
		}

		fields := strings.Fields(out)
		if len(fields) == 0 {
			return "", fmt.Errorf("git ls-remote %s HEAD: no revision", repo)
		}

		return fields[0], nil
	}).Node()
}

// LocalRevisionData reads the local checked-out revision of dir as a data node.
// It returns "" when dir is not a git repository; other errors propagate.
func LocalRevisionData(dirInput input.Input[string]) *reconcile.Node[string] {
	if dirInput == nil {
		panic("git: LocalRevisionData: nil dir")
	}

	return reconcile.Data("git.local-revision:"+dirInput.Name(), func(ctx context.Context) (string, error) {
		dir, err := dirInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("git", "-C", dir, "rev-parse", "HEAD")).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 128) {
				return "", err
			}

			return "", nil
		}

		return strings.TrimSpace(out), nil
	}).Node()
}

// TrackingRevisionData reads dir's tracked upstream revision as a data node
// (dry-run safe, memoized once per run). It returns "" when the branch has no
// upstream; other errors propagate.
func TrackingRevisionData(dirInput input.Input[string]) *reconcile.Node[string] {
	if dirInput == nil {
		panic("git: TrackingRevisionData: nil dir")
	}

	return reconcile.Data("git.tracking-revision:"+dirInput.Name(), func(ctx context.Context) (string, error) {
		dir, err := dirInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("git", "-C", dir, "rev-parse", "@{u}")).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 128) {
				return "", err
			}

			return "", nil
		}

		return strings.TrimSpace(out), nil
	}).Node()
}

// Clone ensures repo is cloned at dir (creating the parent directory). It is
// a resource builder: chain ApplyNeeds/Notify and call Node.
func Clone(repoInput, dirInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	if repoInput == nil {
		panic("git: Clone: nil repo")
	}
	if dirInput == nil {
		panic("git: Clone: nil dir")
	}

	parentPath := input.Map(dirInput, func(ctx context.Context, dir string) (string, error) {
		return path.Dir(dir), nil
	})
	parent := modules.Dir(parentPath).Build().Node()

	return reconcile.ResourceBool("git.clone:"+dirInput.Name(),
		func(ctx context.Context) (bool, error) {
			dir, err := dirInput.Value(ctx)
			if err != nil {
				return false, err
			}

			return modules.DirExistsData(input.Static(path.Join(dir, ".git"))).Value(ctx)
		},
		func(ctx context.Context) error {
			repo, err := repoInput.Value(ctx)
			if err != nil {
				return err
			}
			dir, err := dirInput.Value(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("git", "clone", repo, dir).RunErr()
		}).
		CheckNeeds(parent)
}

// Fetch ensures the remote-tracking refs of the repository at dir are fresh,
// by fetching when the tracked upstream is behind remote. remote is the desired
// revision (see RemoteRevisionData). It assumes the repository exists.
func Fetch(remote *reconcile.Node[string], dirInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	if remote == nil {
		panic("git: Fetch: nil remote")
	}
	if dirInput == nil {
		panic("git: Fetch: nil dir")
	}

	return reconcile.ResourceBool("git.fetch:"+dirInput.Name(),
		func(ctx context.Context) (bool, error) {
			return reconcile.NodesEqual(TrackingRevisionData(dirInput), remote)(ctx)
		},
		func(ctx context.Context) error {
			dir, err := dirInput.Value(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("git", "-C", dir, "fetch", "--prune").RunErr()
		})
}

// Reset ensures the repository at dir is checked out at the tracked remote
// revision, by resetting to the upstream when HEAD is behind. localRevisionProbe
// is the current HEAD (see LocalRevisionData) and remote is the desired revision
// (see RemoteRevisionData). It assumes the repository exists and its upstream
// tracking ref has been fetched (see Fetch). Because its Apply changes HEAD,
// it must read a probe distinct from the observed revision it invalidates (see
// Repo).
func Reset(localRevisionProbe, remote *reconcile.Node[string], dirInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	if localRevisionProbe == nil {
		panic("git: Reset: nil local revision probe")
	}
	if remote == nil {
		panic("git: Reset: nil remote")
	}
	if dirInput == nil {
		panic("git: Reset: nil dir")
	}

	return reconcile.ResourceBool("git.reset:"+dirInput.Name(),
		reconcile.NodesEqual(localRevisionProbe, remote),
		func(ctx context.Context) error {
			dir, err := dirInput.Value(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("git", "-C", dir, "reset", "--hard", "@{u}").RunErr()
		})
}

// RepoGroup is a git repository as a small graph of nodes. It embeds the cover
// node, a group of all the parts, so it can be read directly: reading it
// runs all parts. The cover is not memoized and owns nothing; start a new Run
// (or invalidate RemoteRevision/LocalRevisionProbe) to re-check the remote. The
// checked-out revision is LocalRevision.
//
// The local revision is read twice: LocalRevisionProbe is the HEAD Reset probes
// (it must be distinct from the observed revision the mutator invalidates), and
// LocalRevision is the observed one. Reset invalidates LocalRevision after it
// applies and the cover reads it last, so reading it re-probes.
type RepoGroup struct {
	*reconcile.Node[reconcile.Void]

	RemoteRevision     *reconcile.Node[string]
	LocalRevisionProbe *reconcile.Node[string]
	LocalRevision      *reconcile.Node[string]
	Clone              *reconcile.Node[bool]
	Fetch              *reconcile.Node[bool]
	Reset              *reconcile.Node[bool]
}

// RepoBuilder configures a repository cloned at dir and checked out at the
// remote HEAD.
type RepoBuilder struct {
	repoInput input.Input[string]
	dirInput  input.Input[string]
}

// Repo starts a repository builder.
func Repo(repoInput, dirInput input.Input[string]) *RepoBuilder {
	return &RepoBuilder{repoInput: repoInput, dirInput: dirInput}
}

// Build groups the git data nodes into a repository cloned at dir and checked
// out at the remote HEAD. The repo cover yields Void; read LocalRevision for
// the checked-out revision. Reset invalidates the observed LocalRevision, so
// reading it after the cover reflects the post-reset checkout.
func (b *RepoBuilder) Build() *RepoGroup {
	if b.repoInput == nil {
		panic("git: Repo: nil repo")
	}
	if b.dirInput == nil {
		panic("git: Repo: nil dir")
	}

	repoInput, dirInput := b.repoInput, b.dirInput
	remoteRevision := RemoteRevisionData(repoInput)
	localRevisionProbe := LocalRevisionData(dirInput)
	localRevision := localRevisionProbe.Clone()

	clone := Clone(repoInput, dirInput).Node()
	fetch := Fetch(remoteRevision, dirInput).CheckNeeds(clone).Node()
	reset := Reset(localRevisionProbe, remoteRevision, dirInput).
		CheckNeeds(fetch).
		ApplyInvalidates(localRevision).
		Node()

	node := reconcile.Group("git.repo:"+dirInput.Name()).
		Contains(clone, fetch, reset).
		Contains(remoteRevision, localRevisionProbe).
		Contains(localRevision).
		Node()

	return &RepoGroup{
		Node:               node,
		RemoteRevision:     remoteRevision,
		LocalRevisionProbe: localRevisionProbe,
		LocalRevision:      localRevision,
		Clone:              clone,
		Fetch:              fetch,
		Reset:              reset,
	}
}
