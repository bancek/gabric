package shelltest

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func hostRoot() string {
	if runtime.GOOS != "windows" {
		return "/"
	}

	volume := filepath.VolumeName(os.TempDir())
	if volume == "" {
		volume = "C:"
	}

	return volume + string(filepath.Separator)
}

func hostDir(dir string) string {
	dir = virtualDir(dir)

	if runtime.GOOS != "windows" {
		return dir
	}

	rel := strings.TrimPrefix(dir, "/")
	if rel == "" {
		return hostRoot()
	}

	return filepath.Join(hostRoot(), filepath.FromSlash(rel))
}

func virtualDir(dir string) string {
	if dir == "" {
		return "/"
	}

	if runtime.GOOS == "windows" {
		dir = filepath.ToSlash(dir)
		if volume := filepath.VolumeName(dir); volume != "" {
			dir = strings.TrimPrefix(dir, volume)
		}
	}

	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}

	return path.Clean(dir)
}

func virtualName(name string) string {
	if runtime.GOOS != "windows" || name == "" {
		return name
	}

	if filepath.VolumeName(name) == "" && !strings.HasPrefix(name, `\`) {
		return name
	}

	return virtualDir(name)
}
