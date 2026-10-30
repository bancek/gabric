package gabric

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

const statScript = `if [ ! -e "$1" ]; then
	printf 'missing:\n'
	exit 0
fi

case "$(uname -s 2>/dev/null)" in
Darwin|FreeBSD|OpenBSD|NetBSD|DragonFly|MidnightBSD)
	out=$(stat -L -f '%p:%Su:%Sg' "$1")
	prefix=bsd
	;;
*)
	out=$(stat -L -c '%f:%U:%G' "$1")
	prefix=gnu
	;;
esac

if [ -z "$out" ]; then
	exit 1
fi

printf '%s:%s\n' "$prefix" "$out"`

// Stat reads metadata for a remote path. Use Remote(ctx).Stat(path) to build the
// operation and Run/RunErr to perform it. It follows symlinks.
func (o *RemoteOps) Stat(path string) *StatOps {
	return newStat(o.ctx, runRemoteStat, path)
}

func runRemoteStat(ctx context.Context, call statCall) (FileStat, error) {
	return GetRemoteBackend(ctx).Stat(withoutRemoteWorkingDir(ctx), StatRequest{
		Path: ResolveRemotePath(ctx, call.path),
	})
}

func (realRemoteBackend) Stat(ctx context.Context, req StatRequest) (FileStat, error) {
	e := Remote(ctx).Exec("sh", "-c", statScript, "sh", req.Path).Capture()
	if err := e.RunErr(); err != nil {
		return FileStat{}, err
	}

	stat, err := parseFileStat(e.StdoutStr())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return FileStat{}, &fs.PathError{Op: "stat", Path: req.Path, Err: fs.ErrNotExist}
		}

		return FileStat{}, fmt.Errorf("stat %s: %w", req.Path, err)
	}

	return stat, nil
}

func parseFileStat(out string) (FileStat, error) {
	line := strings.TrimSpace(out)

	if strings.HasPrefix(line, "missing:") {
		return FileStat{}, fs.ErrNotExist
	}

	base := 16

	switch {
	case strings.HasPrefix(line, "gnu:"):
		line = strings.TrimPrefix(line, "gnu:")
	case strings.HasPrefix(line, "bsd:"):
		line = strings.TrimPrefix(line, "bsd:")
		base = 8
	default:
		return FileStat{}, fmt.Errorf("unexpected output %q", out)
	}

	fields := strings.SplitN(line, ":", 3)
	if len(fields) != 3 {
		return FileStat{}, fmt.Errorf("unexpected output %q", out)
	}

	raw, err := strconv.ParseUint(fields[0], base, 32)
	if err != nil {
		return FileStat{}, fmt.Errorf("parse mode %q: %w", fields[0], err)
	}

	return FileStat{
		Mode:  unixFileMode(uint32(raw)),
		User:  fields[1],
		Group: fields[2],
	}, nil
}

func unixFileMode(raw uint32) fs.FileMode {
	mode := fs.FileMode(raw & 0o777)

	if raw&0o4000 != 0 {
		mode |= fs.ModeSetuid
	}
	if raw&0o2000 != 0 {
		mode |= fs.ModeSetgid
	}
	if raw&0o1000 != 0 {
		mode |= fs.ModeSticky
	}

	switch raw & 0o170000 {
	case 0o040000:
		mode |= fs.ModeDir
	case 0o120000:
		mode |= fs.ModeSymlink
	case 0o010000:
		mode |= fs.ModeNamedPipe
	case 0o140000:
		mode |= fs.ModeSocket
	case 0o060000:
		mode |= fs.ModeDevice
	case 0o020000:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	}

	return mode
}
