//go:build !unix

package gabric

import "io/fs"

func fileOwner(fs.FileInfo) (string, string) {
	return "", ""
}
