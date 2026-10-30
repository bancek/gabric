package service

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
)

func TestManagerData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll("/run/systemd/system", 0o755)

	if got := gt.RunNode(gt.Context(), ManagerData()); got != ManagerSystemd {
		t.Errorf("ManagerData() = %v, want %v", got, ManagerSystemd)
	}
}

func TestManagerDataUnsupported(t *testing.T) {
	gt := gabrictest.New(t)

	if _, err := gt.RunNodeErr(gt.Context(), ManagerData()); err == nil {
		t.Error("ManagerData() error = nil, want error")
	}
}
