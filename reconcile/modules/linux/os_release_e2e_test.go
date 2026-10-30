package linux_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestOSReleaseE2E(t *testing.T) {
	moduletest.ForEachLinuxTarget(t, func(g *moduletest.Target) {
		t := g.T()

		release, err := g.RunNodeErr(g.Context(), linux.OSReleaseData())
		if err != nil {
			t.Fatalf("OSReleaseData(): %v", err)
		}

		image := g.Container().Image().Name()

		switch {
		case strings.HasPrefix(image, "alpine"):
			if release.ID != "alpine" {
				t.Errorf("ID = %q, want alpine", release.ID)
			}
		case strings.HasPrefix(image, "ubuntu"):
			if release.ID != "ubuntu" {
				t.Errorf("ID = %q, want ubuntu", release.ID)
			}
			if !release.IsLike("debian") {
				t.Errorf("IsLike(debian) = false for %+v", release)
			}
			if release.VersionCodename == "" {
				t.Error("VersionCodename is empty")
			}
		default:
			t.Fatalf("unexpected target image %q", image)
		}

		if release.Name == "" {
			t.Error("Name is empty")
		}
		if release.VersionID == "" {
			t.Error("VersionID is empty")
		}
	})
}
