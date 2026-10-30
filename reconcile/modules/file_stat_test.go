package modules

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const fileStatPath = "/srv/stat"

func TestFileStatData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)
	gt.Remote().SetOwner(fileStatPath, "testuser", "staff")

	stat, err := gt.RunNodeErr(gt.Context(), FileStatData(input.Static(fileStatPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}

	if !stat.Exists {
		t.Error("Exists = false, want true")
	}
	if stat.Mode.Perm() != 0o640 {
		t.Errorf("Mode.Perm() = %v, want %v", stat.Mode.Perm(), 0o640)
	}
	if stat.User != "testuser" || stat.Group != "staff" {
		t.Errorf("owner = %q:%q, want testuser:staff", stat.User, stat.Group)
	}

	gt.Remote().AssertCalls(gabrictest.Call{Stat: &gabric.StatRequest{Path: fileStatPath}})
}

func TestFileStatDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	stat, err := gt.RunNodeErr(gt.Context(), FileStatData(input.Static(fileStatPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if stat.Exists {
		t.Error("Exists = true, want false")
	}

	gt.Remote().AssertCalls(gabrictest.Call{Stat: &gabric.StatRequest{Path: fileStatPath}})
}

func TestFileModeMatchesData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)

	stat := FileStatData(input.Static(fileStatPath))

	if !gt.RunNode(gt.Context(), FileModeMatchesData(stat, FileMode(0o640))) {
		t.Error("FileModeMatchesData(0o640) = false, want true")
	}
}

func TestFileModeMatchesDataDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o600)

	if gt.RunNode(gt.Context(), FileModeMatchesData(FileStatData(input.Static(fileStatPath)), FileMode(0o640))) {
		t.Error("FileModeMatchesData(0o640) = true, want false")
	}
}

func TestFileModeMatchesDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	if gt.RunNode(gt.Context(), FileModeMatchesData(FileStatData(input.Static(fileStatPath)), FileMode(0o640))) {
		t.Error("FileModeMatchesData(missing) = true, want false")
	}
}

func TestFileOwnerMatchesData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)
	gt.Remote().SetOwner(fileStatPath, "testuser", "staff")

	stat := FileStatData(input.Static(fileStatPath))

	if !gt.RunNode(gt.Context(), FileOwnerMatchesData(stat, input.Static("testuser"), input.Static("staff"))) {
		t.Error("FileOwnerMatchesData(testuser:staff) = false, want true")
	}
}

func TestFileOwnerMatchesDataWildcards(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)
	gt.Remote().SetOwner(fileStatPath, "testuser", "staff")

	stat := FileStatData(input.Static(fileStatPath))

	if !gt.RunNode(gt.Context(), FileOwnerMatchesData(stat, input.Static("testuser"), input.Static(""))) {
		t.Error("FileOwnerMatchesData(testuser:) = false, want true")
	}
	if !gt.RunNode(gt.Context(), FileOwnerMatchesData(stat, input.Static(""), input.Static("staff"))) {
		t.Error("FileOwnerMatchesData(:staff) = false, want true")
	}
}

func TestFileOwnerMatchesDataDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)
	gt.Remote().SetOwner(fileStatPath, "testuser", "staff")

	stat := FileStatData(input.Static(fileStatPath))

	if gt.RunNode(gt.Context(), FileOwnerMatchesData(stat, input.Static("other"), input.Static("staff"))) {
		t.Error("FileOwnerMatchesData(other:staff) = true, want false")
	}
	if gt.RunNode(gt.Context(), FileOwnerMatchesData(stat, input.Static("testuser"), input.Static("other"))) {
		t.Error("FileOwnerMatchesData(testuser:other) = true, want false")
	}
}

func TestFileOwnerMatchesDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	if gt.RunNode(gt.Context(), FileOwnerMatchesData(FileStatData(input.Static(fileStatPath)), input.Static("testuser"), input.Static("staff"))) {
		t.Error("FileOwnerMatchesData(missing) = true, want false")
	}
}

func TestFileStatSharedStat(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileStatPath, []byte("x"), 0o640)
	gt.Remote().SetOwner(fileStatPath, "testuser", "staff")

	stat := FileStatData(input.Static(fileStatPath))
	checks := reconcile.Group("checks").Contains(
		FileModeMatchesData(stat, FileMode(0o640)),
		FileOwnerMatchesData(stat, input.Static("testuser"), input.Static("staff")),
	).Node()

	gt.RunNode(gt.Context(), checks)

	gt.Remote().AssertCalls(gabrictest.Call{Stat: &gabric.StatRequest{Path: fileStatPath}})
}
