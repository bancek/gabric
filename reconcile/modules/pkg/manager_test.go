package pkg

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
)

func TestManagerData(t *testing.T) {
	tests := []struct {
		name    string
		release string
		want    Manager
	}{
		{"ubuntu", "ID=ubuntu\nID_LIKE=debian\n", ManagerApt},
		{"debian", "ID=debian\n", ManagerApt},
		{"debian-like", "ID=custom\nID_LIKE=\"debian ubuntu\"\n", ManagerApt},
		{"alpine", "ID=alpine\n", ManagerApk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().WriteFile("/etc/os-release", []byte(tt.release), 0o644)

			if got := gt.RunNode(gt.Context(), ManagerData()); got != tt.want {
				t.Errorf("ManagerData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManagerDataUnsupported(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=fedora\n"), 0o644)

	if _, err := gt.RunNodeErr(gt.Context(), ManagerData()); err == nil {
		t.Error("ManagerData() error = nil, want error")
	}
}
