package gabric

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

// ownerSpec renders user and group as a chown argument ("user:group", "user",
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

// modeString renders a file mode as an octal string accepted by chmod/install
// (e.g. 0644 -> "644").
func modeString(mode os.FileMode) string {
	return fmt.Sprintf("%o", mode.Perm())
}

// localChown changes the file owner by name. Empty user/group leave that id
// unchanged.
func localChown(path, owner, group string) error {
	if owner == "" && group == "" {
		return nil
	}

	uid, gid := -1, -1

	if owner != "" {
		u, err := user.Lookup(owner)
		if err != nil {
			return err
		}

		uid, err = strconv.Atoi(u.Uid)
		if err != nil {
			return err
		}
	}

	if group != "" {
		g, err := user.LookupGroup(group)
		if err != nil {
			return err
		}

		gid, err = strconv.Atoi(g.Gid)
		if err != nil {
			return err
		}
	}

	return os.Chown(path, uid, gid)
}
