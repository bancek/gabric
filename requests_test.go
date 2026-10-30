package gabric

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetRequestEqual(t *testing.T) {
	if !(GetRequest{Path: "/a"}).Equal(GetRequest{Path: "/a"}) {
		t.Error("Equal() = false, want true")
	}
	if (GetRequest{Path: "/a"}).Equal(GetRequest{Path: "/b"}) {
		t.Error("Equal() = true, want false")
	}
}

func TestExistsRequestEqual(t *testing.T) {
	if !(ExistsRequest{Path: "/a", Kind: FileKindFile}).Equal(ExistsRequest{Path: "/a", Kind: FileKindFile}) {
		t.Error("Equal() = false, want true")
	}
	if (ExistsRequest{Path: "/a", Kind: FileKindFile}).Equal(ExistsRequest{Path: "/a", Kind: FileKindDir}) {
		t.Error("Equal() = true, want false")
	}
}

func TestStatRequestEqual(t *testing.T) {
	if !(StatRequest{Path: "/a"}).Equal(StatRequest{Path: "/a"}) {
		t.Error("Equal() = false, want true")
	}
	if (StatRequest{Path: "/a"}).Equal(StatRequest{Path: "/b"}) {
		t.Error("Equal() = true, want false")
	}
}

func TestMkdirRequestEqual(t *testing.T) {
	a := MkdirRequest{Path: "/a", Recursive: true, Mode: 0o700, User: "root", Group: "root"}
	b := a

	if !a.Equal(b) {
		t.Error("Equal() = false, want true")
	}

	b.Mode = 0o755
	if a.Equal(b) {
		t.Error("Equal() = true, want false")
	}
}

func TestRemoveRequestEqual(t *testing.T) {
	a := RemoveRequest{Path: "/a", Recursive: true, Force: true}
	b := a

	if !a.Equal(b) {
		t.Error("Equal() = false, want true")
	}

	b.Force = false
	if a.Equal(b) {
		t.Error("Equal() = true, want false")
	}
}

func TestPutRequestEqual(t *testing.T) {
	a := PutRequest{
		Path:         "/a",
		Content:      bytes.NewBufferString("hello"),
		Mode:         0o600,
		User:         "root",
		Group:        "root",
		EnsureParent: true,
	}
	b := PutRequest{
		Path:         "/a",
		Content:      bytes.NewBufferString("hello"),
		Mode:         0o600,
		User:         "root",
		Group:        "root",
		EnsureParent: true,
	}

	if !a.Equal(b) {
		t.Error("Equal() = false, want true")
	}

	b.Content = bytes.NewBufferString("other")
	if a.Equal(b) {
		t.Error("Equal() = true with different content, want false")
	}
}

func TestPutRequestEqualContent(t *testing.T) {
	base := PutRequest{Path: "/a"}

	tests := []struct {
		name string
		a    *bytes.Buffer
		b    *bytes.Buffer
		want bool
	}{
		{"both nil", nil, nil, true},
		{"one nil", bytes.NewBufferString("x"), nil, false},
		{"same", bytes.NewBufferString("x"), bytes.NewBufferString("x"), true},
		{"different", bytes.NewBufferString("x"), bytes.NewBufferString("y"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := base, base
			if tt.a != nil {
				a.Content = tt.a
			}
			if tt.b != nil {
				b.Content = tt.b
			}

			if got := a.Equal(b); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPutRequestEqualNonBufferContent(t *testing.T) {
	a := PutRequest{Path: "/a", Content: strings.NewReader("hello")}
	b := PutRequest{Path: "/a", Content: strings.NewReader("hello")}

	if a.Equal(b) {
		t.Error("Equal() = true with non-buffer content, want false")
	}
}
