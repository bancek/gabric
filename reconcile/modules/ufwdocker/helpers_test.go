package ufwdocker

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
)

func existsCall(path string, kind gabric.FileKind) gabrictest.Call {
	return gabrictest.Call{Exists: &gabric.ExistsRequest{Path: path, Kind: kind}}
}

func registerCurlDownload(gt *gabrictest.Test, tmp string, content []byte) {
	gt.Remote().Handle("curl", func(gabrictest.Command) gabrictest.Result {
		gt.Remote().WriteFile(tmp, content, 0o644)

		return gabrictest.Result{}
	})
}
