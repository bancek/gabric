package httpfile

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
)

func statCall(path string) gabrictest.Call {
	return gabrictest.Call{Stat: &gabric.StatRequest{Path: path}}
}

func existsCall(path string, kind gabric.FileKind) gabrictest.Call {
	return gabrictest.Call{Exists: &gabric.ExistsRequest{Path: path, Kind: kind}}
}

func registerCurlDownload(gt *gabrictest.Test, tmp string, content []byte) {
	gt.Remote().Handle("curl", func(gabrictest.Command) gabrictest.Result {
		gt.Remote().WriteFile(tmp, content, 0o644)

		return gabrictest.Result{}
	})
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

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)

	return hex.EncodeToString(sum[:])
}

func httpFileMkdir(path string) gabrictest.Call {
	return gabrictest.Call{Mkdir: &gabric.MkdirRequest{
		Path:      path,
		Recursive: true,
	}}
}
