package modules

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
)

func statCall(path string) gabrictest.Call {
	return gabrictest.Call{Stat: &gabric.StatRequest{Path: path}}
}

func getCall(path string) gabrictest.Call {
	return gabrictest.Call{Get: &gabric.GetRequest{Path: path}}
}

func existsCall(path string, kind gabric.FileKind) gabrictest.Call {
	return gabrictest.Call{Exists: &gabric.ExistsRequest{Path: path, Kind: kind}}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic", name)
		}
	}()

	fn()
}
