//go:build unix

package gabric

import (
	"io/fs"
	"os/user"
	"strconv"
	"syscall"
)

func fileOwner(info fs.FileInfo) (string, string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}

	return lookupUserName(uint32(stat.Uid)), lookupGroupName(uint32(stat.Gid))
}

func lookupUserName(uid uint32) string {
	id := strconv.FormatUint(uint64(uid), 10)

	user, err := user.LookupId(id)
	if err != nil {
		return id
	}

	return user.Username
}

func lookupGroupName(gid uint32) string {
	id := strconv.FormatUint(uint64(gid), 10)

	group, err := user.LookupGroupId(id)
	if err != nil {
		return id
	}

	return group.Name
}
