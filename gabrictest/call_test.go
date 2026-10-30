package gabrictest

import (
	"bytes"
	"slices"
	"testing"

	"github.com/bancek/gabric"
)

func TestCallClone(t *testing.T) {
	original := Call{
		Put: &gabric.PutRequest{
			Path:    "/a",
			Content: bytes.NewBufferString("x"),
		},
	}

	clone := original.Clone()
	if !clone.Equal(original) {
		t.Error("Clone().Equal(original) = false, want true")
	}

	original.Put.Path = "/b"
	original.Put.Content.(*bytes.Buffer).WriteString("y")

	want := Call{Put: &gabric.PutRequest{Path: "/a", Content: bytes.NewBufferString("x")}}
	if !clone.Equal(want) {
		t.Error("clone changed after mutating the original")
	}

	get := Call{Get: &gabric.GetRequest{Path: "/a"}}
	getClone := get.Clone()
	get.Get.Path = "/b"

	if getClone.Get.Path != "/a" {
		t.Errorf("Get clone path = %q, want %q", getClone.Get.Path, "/a")
	}

	stat := Call{Stat: &gabric.StatRequest{Path: "/a"}}
	statClone := stat.Clone()
	stat.Stat.Path = "/b"

	if statClone.Stat.Path != "/a" {
		t.Errorf("Stat clone path = %q, want %q", statClone.Stat.Path, "/a")
	}

	remove := Call{Remove: &gabric.RemoveRequest{Path: "/a", Recursive: true}}
	removeClone := remove.Clone()
	remove.Remove.Path = "/b"
	remove.Remove.Recursive = false

	if removeClone.Remove.Path != "/a" || !removeClone.Remove.Recursive {
		t.Errorf("Remove clone = %+v, want unchanged request", removeClone.Remove)
	}

	list := Call{List: &gabric.ListRequest{Path: "/a"}}
	listClone := list.Clone()
	list.List.Path = "/b"

	if listClone.List.Path != "/a" {
		t.Errorf("List clone path = %q, want %q", listClone.List.Path, "/a")
	}

	rsync := Call{Rsync: &gabric.RsyncRequest{
		LocalPath:  "./a",
		RemotePath: "/a",
		Flags:      []string{"-a"},
	}}
	rsyncClone := rsync.Clone()
	rsync.Rsync.LocalPath = "./b"
	rsync.Rsync.Flags[0] = "-b"

	if rsyncClone.Rsync.LocalPath != "./a" || !slices.Equal(rsyncClone.Rsync.Flags, []string{"-a"}) {
		t.Errorf("Rsync clone = %+v, want unchanged local path and flags", rsyncClone.Rsync)
	}
}

func TestCallEqual(t *testing.T) {
	tests := []struct {
		name string
		a    Call
		b    Call
		want bool
	}{
		{
			name: "put",
			a:    Call{Put: &gabric.PutRequest{Path: "/a", Content: bytes.NewBufferString("x")}},
			b:    Call{Put: &gabric.PutRequest{Path: "/a", Content: bytes.NewBufferString("x")}},
			want: true,
		},
		{
			name: "put different content",
			a:    Call{Put: &gabric.PutRequest{Path: "/a", Content: bytes.NewBufferString("x")}},
			b:    Call{Put: &gabric.PutRequest{Path: "/a", Content: bytes.NewBufferString("y")}},
			want: false,
		},
		{
			name: "get",
			a:    Call{Get: &gabric.GetRequest{Path: "/a"}},
			b:    Call{Get: &gabric.GetRequest{Path: "/a"}},
			want: true,
		},
		{
			name: "mkdir",
			a:    Call{Mkdir: &gabric.MkdirRequest{Path: "/a", Recursive: true}},
			b:    Call{Mkdir: &gabric.MkdirRequest{Path: "/a", Recursive: true}},
			want: true,
		},
		{
			name: "remove",
			a:    Call{Remove: &gabric.RemoveRequest{Path: "/a", Recursive: true, Force: true}},
			b:    Call{Remove: &gabric.RemoveRequest{Path: "/a", Recursive: true, Force: true}},
			want: true,
		},
		{
			name: "remove different flags",
			a:    Call{Remove: &gabric.RemoveRequest{Path: "/a", Recursive: true}},
			b:    Call{Remove: &gabric.RemoveRequest{Path: "/a", Force: true}},
			want: false,
		},
		{
			name: "exists",
			a:    Call{Exists: &gabric.ExistsRequest{Path: "/a", Kind: gabric.FileKindFile}},
			b:    Call{Exists: &gabric.ExistsRequest{Path: "/a", Kind: gabric.FileKindFile}},
			want: true,
		},
		{
			name: "stat",
			a:    Call{Stat: &gabric.StatRequest{Path: "/a"}},
			b:    Call{Stat: &gabric.StatRequest{Path: "/a"}},
			want: true,
		},
		{
			name: "stat different path",
			a:    Call{Stat: &gabric.StatRequest{Path: "/a"}},
			b:    Call{Stat: &gabric.StatRequest{Path: "/b"}},
			want: false,
		},
		{
			name: "list",
			a:    Call{List: &gabric.ListRequest{Path: "/a"}},
			b:    Call{List: &gabric.ListRequest{Path: "/a"}},
			want: true,
		},
		{
			name: "list different path",
			a:    Call{List: &gabric.ListRequest{Path: "/a"}},
			b:    Call{List: &gabric.ListRequest{Path: "/b"}},
			want: false,
		},
		{
			name: "rsync",
			a: Call{Rsync: &gabric.RsyncRequest{
				LocalPath:  "./a",
				RemotePath: "/a",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-a"},
			}},
			b: Call{Rsync: &gabric.RsyncRequest{
				LocalPath:  "./a",
				RemotePath: "/a",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-a"},
			}},
			want: true,
		},
		{
			name: "rsync different flags",
			a: Call{Rsync: &gabric.RsyncRequest{
				LocalPath:  "./a",
				RemotePath: "/a",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-a"},
			}},
			b: Call{Rsync: &gabric.RsyncRequest{
				LocalPath:  "./a",
				RemotePath: "/a",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-b"},
			}},
			want: false,
		},
		{
			name: "different variants",
			a:    Call{Get: &gabric.GetRequest{Path: "/a"}},
			b:    Call{Mkdir: &gabric.MkdirRequest{Path: "/a"}},
			want: false,
		},
		{
			name: "multiple variants",
			a: Call{
				Get:   &gabric.GetRequest{Path: "/a"},
				Mkdir: &gabric.MkdirRequest{Path: "/a"},
			},
			b:    Call{Get: &gabric.GetRequest{Path: "/a"}},
			want: false,
		},
		{
			name: "empty",
			a:    Call{},
			b:    Call{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
			if got := tt.b.Equal(tt.a); got != tt.want {
				t.Errorf("reverse Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
