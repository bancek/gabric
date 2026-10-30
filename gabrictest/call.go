package gabrictest

import (
	"bytes"

	"github.com/bancek/gabric"
)

// Call is one high-level backend operation. Exactly one field is non-nil.
type Call struct {
	Put    *gabric.PutRequest
	Get    *gabric.GetRequest
	Mkdir  *gabric.MkdirRequest
	Remove *gabric.RemoveRequest
	Exists *gabric.ExistsRequest
	Stat   *gabric.StatRequest
	List   *gabric.ListRequest
	Rsync  *gabric.RsyncRequest
}

// Equal reports whether c and other describe the same call.
func (c Call) Equal(other Call) bool {
	if c.variants() != 1 || other.variants() != 1 {
		return false
	}

	switch {
	case c.Put != nil:
		return other.Put != nil && c.Put.Equal(*other.Put)
	case c.Get != nil:
		return other.Get != nil && c.Get.Equal(*other.Get)
	case c.Mkdir != nil:
		return other.Mkdir != nil && c.Mkdir.Equal(*other.Mkdir)
	case c.Remove != nil:
		return other.Remove != nil && c.Remove.Equal(*other.Remove)
	case c.Exists != nil:
		return other.Exists != nil && c.Exists.Equal(*other.Exists)
	case c.Stat != nil:
		return other.Stat != nil && c.Stat.Equal(*other.Stat)
	case c.List != nil:
		return other.List != nil && c.List.Equal(*other.List)
	case c.Rsync != nil:
		return other.Rsync != nil && c.Rsync.Equal(*other.Rsync)
	default:
		return false
	}
}

// Clone returns a deep copy of c. Put content buffers are copied; other reader
// types are copied as-is.
func (c Call) Clone() Call {
	var clone Call

	if c.Put != nil {
		req := *c.Put

		if buffer, ok := req.Content.(*bytes.Buffer); ok {
			req.Content = bytes.NewBuffer(bytes.Clone(buffer.Bytes()))
		}

		clone.Put = &req
	}
	if c.Get != nil {
		req := *c.Get
		clone.Get = &req
	}
	if c.Mkdir != nil {
		req := *c.Mkdir
		clone.Mkdir = &req
	}
	if c.Remove != nil {
		req := *c.Remove
		clone.Remove = &req
	}
	if c.Exists != nil {
		req := *c.Exists
		clone.Exists = &req
	}
	if c.Stat != nil {
		req := *c.Stat
		clone.Stat = &req
	}
	if c.List != nil {
		req := *c.List
		clone.List = &req
	}
	if c.Rsync != nil {
		req := c.Rsync.Clone()
		clone.Rsync = &req
	}

	return clone
}

func (c Call) variants() int {
	count := 0

	if c.Put != nil {
		count++
	}
	if c.Get != nil {
		count++
	}
	if c.Mkdir != nil {
		count++
	}
	if c.Remove != nil {
		count++
	}
	if c.Exists != nil {
		count++
	}
	if c.Stat != nil {
		count++
	}
	if c.List != nil {
		count++
	}
	if c.Rsync != nil {
		count++
	}

	return count
}
