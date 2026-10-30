package git_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/git"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestGitE2E(t *testing.T) {
	moduletest.ForEachToolsTarget(t, func(g *moduletest.Target) {
		t := g.T()

		origin := g.Path("origin.git")
		work := g.Path("work")
		checkout := g.Path("checkout")

		gitRun(t, g, "init", "--bare", "-b", "main", origin)
		gitRun(t, g, "init", "-b", "main", work)
		writeGitFile(t, g, work+"/README.md", "one\n")
		gitRun(t, g, "-C", work, "add", "README.md")
		gitCommit(t, g, work, "one")
		gitRun(t, g, "-C", work, "remote", "add", "origin", origin)
		gitRun(t, g, "-C", work, "push", "-u", "origin", "main")

		repo := git.Repo(input.Static(origin), input.Static(checkout)).Build()

		moduletest.AssertDryRun(t, g, repo)
		moduletest.AssertConverges(t, g, repo)

		first := strings.Fields(gitRun(t, g, "ls-remote", origin, "HEAD"))[0]
		if got := gitRun(t, g, "-C", checkout, "rev-parse", "HEAD"); got != first {
			t.Errorf("HEAD = %q, want %q", got, first)
		}
		if got := g.Ops().Get(checkout + "/README.md").String(); got != "one\n" {
			t.Errorf("README.md = %q, want %q", got, "one\n")
		}

		remote, err := g.RunNodeErr(g.Context(), git.RemoteRevisionData(input.Static(origin)))
		if err != nil {
			t.Fatalf("RemoteRevisionData(): %v", err)
		}
		if remote != first {
			t.Errorf("RemoteRevisionData() = %q, want %q", remote, first)
		}

		local, err := g.RunNodeErr(g.Context(), git.LocalRevisionData(input.Static(checkout)))
		if err != nil {
			t.Fatalf("LocalRevisionData(): %v", err)
		}
		if local != first {
			t.Errorf("LocalRevisionData() = %q, want %q", local, first)
		}

		tracking, err := g.RunNodeErr(g.Context(), git.TrackingRevisionData(input.Static(checkout)))
		if err != nil {
			t.Fatalf("TrackingRevisionData(): %v", err)
		}
		if tracking != first {
			t.Errorf("TrackingRevisionData() = %q, want %q", tracking, first)
		}

		writeGitFile(t, g, work+"/README.md", "two\n")
		gitRun(t, g, "-C", work, "add", "README.md")
		gitCommit(t, g, work, "two")
		gitRun(t, g, "-C", work, "push", "origin", "main")

		moduletest.AssertConverges(t, g, repo)

		second := strings.Fields(gitRun(t, g, "ls-remote", origin, "HEAD"))[0]
		if second == first {
			t.Fatalf("remote revision did not change")
		}
		if got := gitRun(t, g, "-C", checkout, "rev-parse", "HEAD"); got != second {
			t.Errorf("HEAD = %q, want %q", got, second)
		}
		if got := g.Ops().Get(checkout + "/README.md").String(); got != "two\n" {
			t.Errorf("README.md = %q, want %q", got, "two\n")
		}

		localRevision, err := g.RunNodeErr(g.Context(), repo.LocalRevision)
		if err != nil {
			t.Fatalf("LocalRevision(): %v", err)
		}
		if localRevision != second {
			t.Errorf("LocalRevision() = %q, want %q", localRevision, second)
		}
	})
}

func gitRun(t testing.TB, g *moduletest.Target, args ...string) string {
	t.Helper()

	e := g.Ops().Exec(append([]string{"git"}, args...)...).Capture()
	if err := e.RunErr(); err != nil {
		t.Fatalf("git %s: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), err, e.StdoutStr(), e.StderrStr())
	}

	return strings.TrimSpace(e.StdoutStr())
}

func gitCommit(t testing.TB, g *moduletest.Target, dir, message string) {
	t.Helper()

	gitRun(t, g, "-C", dir,
		"-c", "user.name=modtest",
		"-c", "user.email=modtest@example.com",
		"commit", "-m", message,
	)
}

func writeGitFile(t testing.TB, g *moduletest.Target, path, content string) {
	t.Helper()

	if err := g.Ops().Put(path).EnsureParent().String(content).Mode(0o644).RunErr(); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
