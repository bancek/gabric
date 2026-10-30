package linux

import (
	"slices"
	"testing"

	"github.com/bancek/gabric/gabrictest"
)

func TestOSReleaseData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte(`NAME="Ubuntu"
ID=ubuntu
ID_LIKE="debian ubuntu"
PRETTY_NAME="Ubuntu 26.04 LTS"
VERSION_ID="26.04"
VERSION_CODENAME=resolute
`), 0o644)

	got := gt.RunNode(gt.Context(), OSReleaseData())

	if got.ID != "ubuntu" {
		t.Errorf("ID = %q, want ubuntu", got.ID)
	}
	if want := []string{"debian", "ubuntu"}; !slices.Equal(got.IDLike, want) {
		t.Errorf("IDLike = %v, want %v", got.IDLike, want)
	}
	if got.Name != "Ubuntu" {
		t.Errorf("Name = %q, want Ubuntu", got.Name)
	}
	if got.PrettyName != "Ubuntu 26.04 LTS" {
		t.Errorf("PrettyName = %q, want Ubuntu 26.04 LTS", got.PrettyName)
	}
	if got.VersionID != "26.04" {
		t.Errorf("VersionID = %q, want 26.04", got.VersionID)
	}
	if got.VersionCodename != "resolute" {
		t.Errorf("VersionCodename = %q, want resolute", got.VersionCodename)
	}
}

func TestOSReleaseIsLike(t *testing.T) {
	release := OSRelease{ID: "ubuntu", IDLike: []string{"debian"}}

	if !release.IsLike("ubuntu") {
		t.Error("IsLike(ubuntu) = false, want true")
	}
	if !release.IsLike("debian") {
		t.Error("IsLike(debian) = false, want true")
	}
	if !release.IsLike("alpine", "debian") {
		t.Error("IsLike(alpine, debian) = false, want true")
	}
	if release.IsLike("alpine", "fedora") {
		t.Error("IsLike(alpine, fedora) = true, want false")
	}
}

func TestParseOSRelease(t *testing.T) {
	got := parseOSRelease(`# comment
NAME='Custom Linux'
ID=custom
ID_LIKE="debian ubuntu"
PRETTY_NAME="Custom \"Linux\" \\ Edition"
VERSION_CODENAME="bookworm"
`)

	if got.ID != "custom" {
		t.Errorf("ID = %q, want custom", got.ID)
	}
	if want := []string{"debian", "ubuntu"}; !slices.Equal(got.IDLike, want) {
		t.Errorf("IDLike = %v, want %v", got.IDLike, want)
	}
	if got.Name != "Custom Linux" {
		t.Errorf("Name = %q, want Custom Linux", got.Name)
	}
	if got.PrettyName != `Custom "Linux" \ Edition` {
		t.Errorf("PrettyName = %q, want %q", got.PrettyName, `Custom "Linux" \ Edition`)
	}
	if got.VersionCodename != "bookworm" {
		t.Errorf("VersionCodename = %q, want bookworm", got.VersionCodename)
	}
}
