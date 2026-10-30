package httpfile

import (
	"fmt"
	"os"
	"path"
)

func modeString(mode os.FileMode) string {
	return fmt.Sprintf("%o", mode.Perm())
}

// ownerSpec renders user and group as a chown argument ("linux.user:group", "user",
// ":group"). Both empty yields "".
func ownerSpec(user, group string) string {
	switch {
	case user != "" && group != "":
		return user + ":" + group
	case user != "":
		return user
	case group != "":
		return ":" + group
	default:
		return ""
	}
}

// parentDir returns the parent of p, or "" when there is nothing to create.
func parentDir(p string) string {
	dir := path.Dir(p)

	if dir == "" || dir == "." || dir == "/" {
		return ""
	}

	return dir
}
