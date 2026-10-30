package git

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const (
	gitRepoURL  = "https://example.com/repo.git"
	gitRepoDir  = "/srv/repo"
	gitCloneDir = "/srv/new"
)

func registerGit(gt *gabrictest.Test, remoteRev, localRev, upstreamRev string) {
	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("ls-remote"):
			return gabrictest.Result{Stdout: remoteRev + "\tHEAD\n"}
		case cmd.HasArg("fetch"), cmd.HasArg("reset"), cmd.HasArg("clone"):
			return gabrictest.Result{}
		case cmd.HasArg("@{u}"):
			return gabrictest.Result{Stdout: upstreamRev + "\n"}
		case cmd.HasArg("rev-parse"):
			return gabrictest.Result{Stdout: localRev + "\n"}
		}

		return gabrictest.Result{}
	})
}

func gitDirExists(path string) gabrictest.Call {
	return gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: path,
		Kind: gabric.FileKindDir,
	}}
}

func TestRemoteRevisionData(t *testing.T) {
	gt := gabrictest.New(t)
	registerGit(gt, "abc123", "", "")

	got := gt.RunNode(gt.Context(), RemoteRevisionData(input.Static(gitRepoURL)))
	if got != "abc123" {
		t.Errorf("RemoteRevisionData() = %q, want %q", got, "abc123")
	}

	gt.Remote().AssertCommands(gt.Command("git", "ls-remote", gitRepoURL, "HEAD"))
	gt.Remote().AssertCalls()
}

func TestRemoteRevisionDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("ls-remote") {
			return gabrictest.Result{ExitCode: 1}
		}

		return gabrictest.Result{}
	})

	if _, err := gt.RunNodeErr(gt.Context(), RemoteRevisionData(input.Static(gitRepoURL))); err == nil {
		t.Error("RemoteRevisionData() error = nil, want error")
	}

	gt.Remote().AssertCommands(gt.Command("git", "ls-remote", gitRepoURL, "HEAD"))
	gt.Remote().AssertCalls()
}

func TestLocalRevisionData(t *testing.T) {
	gt := gabrictest.New(t)
	registerGit(gt, "", "abc123", "")

	got := gt.RunNode(gt.Context(), LocalRevisionData(input.Static(gitRepoDir)))
	if got != "abc123" {
		t.Errorf("LocalRevisionData() = %q, want %q", got, "abc123")
	}

	gt.Remote().AssertCommands(gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"))
	gt.Remote().AssertCalls()
}

func TestLocalRevisionDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("rev-parse") {
			return gabrictest.Result{ExitCode: 128}
		}

		return gabrictest.Result{}
	})

	got := gt.RunNode(gt.Context(), LocalRevisionData(input.Static(gitRepoDir)))
	if got != "" {
		t.Errorf("LocalRevisionData() = %q, want empty", got)
	}

	gt.Remote().AssertCommands(gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"))
	gt.Remote().AssertCalls()
}

func TestTrackingRevisionData(t *testing.T) {
	gt := gabrictest.New(t)
	registerGit(gt, "", "", "abc123")

	got := gt.RunNode(gt.Context(), TrackingRevisionData(input.Static(gitRepoDir)))
	if got != "abc123" {
		t.Errorf("TrackingRevisionData() = %q, want %q", got, "abc123")
	}

	gt.Remote().AssertCommands(gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"))
}

func TestTrackingRevisionDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("git", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 128}
	})

	got, err := gt.RunNodeErr(gt.Context(), TrackingRevisionData(input.Static(gitRepoDir)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("TrackingRevisionData() = %q, want empty", got)
	}
}

func TestClone(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll("/srv", 0o755)
	registerGit(gt, "", "", "")

	gt.RunNode(gt.Context(), Clone(input.Static(gitRepoURL), input.Static(gitCloneDir)).Node())

	gt.Remote().AssertCommands(gt.Command("git", "clone", gitRepoURL, gitCloneDir))
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitCloneDir+"/.git"),
	)
}

func TestFetch(t *testing.T) {
	gt := gabrictest.New(t)
	registerGit(gt, "def456", "abc123", "abc123")

	remote := RemoteRevisionData(input.Static(gitRepoURL))
	gt.RunNode(gt.Context(), Fetch(remote, input.Static(gitRepoDir)).Node())

	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "fetch", "--prune"),
	)
	gt.Remote().AssertCalls()
}

func TestReset(t *testing.T) {
	gt := gabrictest.New(t)
	registerGit(gt, "def456", "abc123", "")

	remote := RemoteRevisionData(input.Static(gitRepoURL))
	local := LocalRevisionData(input.Static(gitRepoDir))
	gt.RunNode(gt.Context(), Reset(local, remote, input.Static(gitRepoDir)).Node())

	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "reset", "--hard", "@{u}"),
	)
	gt.Remote().AssertCalls()
}

func TestRepoMatches(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(gitRepoDir+"/.git", 0o755)
	registerGit(gt, "abc123", "abc123", "abc123")

	gt.RunNode(gt.Context(), Repo(input.Static(gitRepoURL), input.Static(gitRepoDir)).Build().Node)

	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
	)
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitRepoDir+"/.git"),
	)
}

func TestRepoDriftConverges(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(gitRepoDir+"/.git", 0o755)

	var (
		mu       sync.Mutex
		head     = "abc123"
		upstream = "abc123"
	)

	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case cmd.HasArg("ls-remote"):
			return gabrictest.Result{Stdout: "def456\tHEAD\n"}
		case cmd.HasArg("fetch"):
			upstream = "def456"

			return gabrictest.Result{}
		case cmd.HasArg("reset"):
			head = "def456"

			return gabrictest.Result{}
		case cmd.HasArg("@{u}"):
			return gabrictest.Result{Stdout: upstream + "\n"}
		case cmd.HasArg("rev-parse"):
			return gabrictest.Result{Stdout: head + "\n"}
		}

		return gabrictest.Result{}
	})

	ctx := gt.Context()
	repo := Repo(input.Static(gitRepoURL), input.Static(gitRepoDir)).Build()

	gt.RunNode(ctx, repo.Node)
	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "fetch", "--prune"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "reset", "--hard", "@{u}"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
	)
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitRepoDir+"/.git"),
	)

	gt.Remote().ResetCommands()
	gt.Remote().ResetCalls()

	// The repository now matches, so a fresh run only probes.
	gt.RunNode(ctx, repo.Node)
	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
	)
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitRepoDir+"/.git"),
	)
}

func TestRepoClone(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll("/srv", 0o755)
	registerGit(gt, "abc123", "abc123", "abc123")

	gt.RunNode(gt.Context(), Repo(input.Static(gitRepoURL), input.Static(gitCloneDir)).Build().Node)

	gt.Remote().AssertCommands(
		gt.Command("git", "clone", gitRepoURL, gitCloneDir),
		gt.Command("git", "-C", gitCloneDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitCloneDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitCloneDir, "rev-parse", "HEAD"),
	)
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitCloneDir+"/.git"),
	)
}

// TestRepoNewRun checks that a new Run re-reads the remote. A group is
// not memoized and has no ownership, so starting a fresh Run is how you force a
// full re-check.
func TestRepoNewRun(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(gitRepoDir+"/.git", 0o755)

	lsRemotes := 0
	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("ls-remote"):
			lsRemotes++

			return gabrictest.Result{Stdout: "abc123\tHEAD\n"}
		case cmd.HasArg("@{u}"):
			return gabrictest.Result{Stdout: "abc123\n"}
		case cmd.HasArg("rev-parse"):
			return gabrictest.Result{Stdout: "abc123\n"}
		}

		return gabrictest.Result{}
	})

	ctx := gt.Context()
	repo := Repo(input.Static(gitRepoURL), input.Static(gitRepoDir)).Build()

	wantCommands := []gabrictest.Command{
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
	}
	wantCalls := []gabrictest.Call{
		gitDirExists("/srv"),
		gitDirExists(gitRepoDir + "/.git"),
	}

	gt.RunNode(ctx, repo.Node)
	gt.Remote().AssertCommands(wantCommands...)
	gt.Remote().AssertCalls(wantCalls...)

	gt.Remote().ResetCommands()
	gt.Remote().ResetCalls()

	gt.RunNode(ctx, repo.Node)
	gt.Remote().AssertCommands(wantCommands...)
	gt.Remote().AssertCalls(wantCalls...)

	if lsRemotes != 2 {
		t.Errorf("ls-remote ran %d times, want 2 (a new run re-checks the remote)", lsRemotes)
	}
}

func TestRepoRefreshesLocalRevision(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(gitRepoDir+"/.git", 0o755)

	var (
		mu   sync.Mutex
		head = "abc123"
	)

	gt.Remote().Handle("git", func(cmd gabrictest.Command) gabrictest.Result {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case cmd.HasArg("ls-remote"):
			return gabrictest.Result{Stdout: "def456\tHEAD\n"}
		case cmd.HasArg("reset"):
			head = "def456"

			return gabrictest.Result{}
		case cmd.HasArg("@{u}"):
			return gabrictest.Result{Stdout: "def456\n"}
		case cmd.HasArg("rev-parse"):
			return gabrictest.Result{Stdout: head + "\n"}
		}

		return gabrictest.Result{}
	})

	ctx := gt.Context()
	repo := Repo(input.Static(gitRepoURL), input.Static(gitRepoDir)).Build()

	var after string

	err := reconcile.Run(ctx, func(ctx context.Context) error {
		if _, err := repo.LocalRevision.Value(ctx); err != nil {
			return err
		}

		if _, err := repo.Value(ctx); err != nil {
			return err
		}

		var err error
		after, err = repo.LocalRevision.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if after != "def456" {
		t.Errorf("repo.LocalRevision after the cover = %q, want %q (invalidated and re-probed)", after, "def456")
	}

	gt.Remote().AssertCommands(
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "@{u}"),
		gt.Command("git", "ls-remote", gitRepoURL, "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
		gt.Command("git", "-C", gitRepoDir, "reset", "--hard", "@{u}"),
		gt.Command("git", "-C", gitRepoDir, "rev-parse", "HEAD"),
	)
	gt.Remote().AssertCalls(
		gitDirExists("/srv"),
		gitDirExists(gitRepoDir+"/.git"),
	)
}

func TestRevisionDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("git", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), LocalRevisionData(input.Static(gitRepoDir))); err == nil {
		t.Error("LocalRevisionData() error = nil, want error")
	}
	if _, err := gt.RunNodeErr(gt.Context(), TrackingRevisionData(input.Static(gitRepoDir))); err == nil {
		t.Error("TrackingRevisionData() error = nil, want error")
	}
}

func TestRevisionDataTransportErrors(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	t.Run("LocalRevisionData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, LocalRevisionData(input.Static(gitRepoDir))); !errors.Is(err, sentinel) {
			t.Errorf("LocalRevisionData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("TrackingRevisionData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, TrackingRevisionData(input.Static(gitRepoDir))); !errors.Is(err, sentinel) {
			t.Errorf("TrackingRevisionData() error = %v, want %v", err, sentinel)
		}
	})
}
