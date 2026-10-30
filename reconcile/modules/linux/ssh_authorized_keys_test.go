package linux

import (
	"bytes"
	"context"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const testAuthorizedKey = "ssh-ed25519 AAAAtest testuser@test"

func TestSSHAuthorizedKeysResourceNames(t *testing.T) {
	keys := SSHAuthorizedKeys(
		input.Static("testuser"),
		input.List(testAuthorizedKey),
	)

	if got, want := keys.Dir.Name(), "modules.dir:testuser"; got != want {
		t.Errorf("Dir.Name() = %q, want %q", got, want)
	}
	if got, want := keys.File.Name(), "modules.file:testuser"; got != want {
		t.Errorf("File.Name() = %q, want %q", got, want)
	}
}

func TestSSHAuthorizedKeysContentData(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{name: "empty", keys: nil, want: ""},
		{name: "one", keys: []string{testAuthorizedKey}, want: testAuthorizedKey + "\n"},
		{
			name: "trims and drops blanks",
			keys: []string{"  " + testAuthorizedKey + "  ", "", "ssh-ed25519 AAAAother other@test"},
			want: testAuthorizedKey + "\nssh-ed25519 AAAAother other@test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reconcile.RunNode(context.Background(), SSHAuthorizedKeysContentData(input.Static(tt.keys)))
			if err != nil {
				t.Fatalf("content: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSSHAuthorizedKeysContentDataNilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for a nil keys input")
		}
	}()

	SSHAuthorizedKeysContentData(nil)
}

func TestSSHAuthorizedKeysNoChange(t *testing.T) {
	gt := gabrictest.New(t)

	content := sshAuthorizedKeysContent([]string{testAuthorizedKey})
	gt.Remote().MkdirAll("/home/testuser/.ssh", 0o700)
	gt.Remote().WriteFile("/home/testuser/.ssh/authorized_keys", content, 0o600)
	gt.Remote().SetOwner("/home/testuser/.ssh", "testuser", "testuser")
	gt.Remote().SetOwner("/home/testuser/.ssh/authorized_keys", "testuser", "testuser")

	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser\n"}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	keys := SSHAuthorizedKeys(input.Static("testuser"), input.List(testAuthorizedKey))
	if keys.Dir == nil || keys.File == nil {
		t.Fatal("SSHAuthorizedKeys() returned nil Dir or File node")
	}
	gt.RunNode(ctx, keys.Node)

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "id", "-gn", "testuser"),
		sudoCommand(gt, "sha256sum", "/home/testuser/.ssh/authorized_keys"),
	)
	gt.Remote().AssertCalls(
		statCall("/home/testuser/.ssh"),
		statCall("/home/testuser/.ssh"),
		statCall("/home/testuser/.ssh/authorized_keys"),
		statCall("/home/testuser/.ssh/authorized_keys"),
	)
}

func TestSSHAuthorizedKeysCreates(t *testing.T) {
	gt := gabrictest.New(t)

	content := sshAuthorizedKeysContent([]string{testAuthorizedKey})
	gt.Remote().MkdirAll("/home/testuser", 0o755)

	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser\n"}
	})

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	keys := SSHAuthorizedKeys(input.Static("testuser"), input.List(testAuthorizedKey))
	if keys.Dir == nil || keys.File == nil {
		t.Fatal("SSHAuthorizedKeys() returned nil Dir or File node")
	}
	gt.RunNode(ctx, keys.Node)

	gt.Remote().AssertCommands(
		sudoCommand(gt, "getent", "passwd", "testuser"),
		sudoCommand(gt, "id", "-gn", "testuser"),
		sudoCommand(gt, "sha256sum", "/home/testuser/.ssh/authorized_keys"),
	)
	gt.Remote().AssertCalls(
		statCall("/home/testuser/.ssh"),
		gabrictest.Call{Mkdir: &gabric.MkdirRequest{
			Path:      "/home/testuser/.ssh",
			Recursive: true,
			Mode:      0o700,
			User:      "testuser",
			Group:     "testuser",
		}},
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: "/home/testuser/.ssh/authorized_keys",
			Kind: gabric.FileKindAny,
		}},
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         "/home/testuser/.ssh/authorized_keys",
			Content:      bytes.NewBuffer(content),
			Mode:         0o600,
			User:         "testuser",
			Group:        "testuser",
			EnsureParent: true,
		}},
	)
}

func TestSSHAuthorizedKeysPathData(t *testing.T) {
	passwd := reconcile.Data("passwd", func(context.Context) (UserPasswd, error) {
		return UserPasswd{Name: "testuser", Home: "/home/testuser", Exists: true}, nil
	}).Node()
	dirNode := SSHAuthorizedKeysDirPathData(passwd)

	dir, err := reconcile.RunNode(context.Background(), dirNode)
	if err != nil {
		t.Fatalf("dir path: %v", err)
	}
	if dir != "/home/testuser/.ssh" {
		t.Errorf("dir path = %q, want /home/testuser/.ssh", dir)
	}

	file, err := reconcile.RunNode(context.Background(), SSHAuthorizedKeysFilePathData(dirNode))
	if err != nil {
		t.Fatalf("file path: %v", err)
	}
	if file != "/home/testuser/.ssh/authorized_keys" {
		t.Errorf("file path = %q, want /home/testuser/.ssh/authorized_keys", file)
	}
}

func TestSSHAuthorizedKeysPathDataMissingUser(t *testing.T) {
	passwd := reconcile.Data("passwd", func(context.Context) (UserPasswd, error) {
		return UserPasswd{}, nil
	}).Node()

	if _, err := reconcile.RunNode(context.Background(), SSHAuthorizedKeysDirPathData(passwd)); err == nil {
		t.Error("dir path error = nil, want error")
	}
}
