package gabrictest

import (
	"path"
	"path/filepath"
	"strings"
)

func virtualPath(p string) string {
	p = filepath.ToSlash(p)

	if volume := filepath.VolumeName(p); volume != "" {
		p = strings.TrimPrefix(p, volume)
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	return path.Clean(p)
}
