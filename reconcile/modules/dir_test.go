package modules

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

const dirPath = "/srv/dir"

func TestDirResourceName(t *testing.T) {
	if got, want := Dir(input.Static(dirPath)).Build().Node().Name(), "modules.dir:"+dirPath; got != want {
		t.Errorf("default name = %q, want %q", got, want)
	}

	got := Dir(input.WithName(input.Static(dirPath), "state")).Build().Node().Name()
	if want := "modules.dir:state"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestDirSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(dirPath, 0o755)

	if !gt.RunNode(gt.Context(), DirSatisfiedData(input.Static(dirPath), FileMode(0o755))) {
		t.Error("DirSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCalls(statCall(dirPath))
}

func TestDirNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(dirPath, 0o755)

	gt.RunNode(gt.Context(), Dir(input.Static(dirPath)).Build().Node())

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: dirPath,
		Kind: gabric.FileKindDir,
	}})
}

func TestDirDrift(t *testing.T) {
	gt := gabrictest.New(t)

	gt.RunNode(gt.Context(), Dir(input.Static(dirPath)).Build().Node())

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: dirPath,
			Kind: gabric.FileKindDir,
		}},
		gabrictest.Call{Mkdir: &gabric.MkdirRequest{
			Path:      dirPath,
			Recursive: true,
		}},
	)
}

func TestDirModeDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(dirPath, 0o755)

	gt.RunNode(gt.Context(), Dir(input.Static(dirPath)).Mode(FileMode(0o700)).Build().Node())

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(
		statCall(dirPath),
		gabrictest.Call{Mkdir: &gabric.MkdirRequest{
			Path:      dirPath,
			Recursive: true,
			Mode:      0o700,
		}},
	)
}
