package linux

import (
	"errors"
	"slices"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestUserPasswdData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000:Test User:/home/testuser:/bin/bash\n"}
	})

	passwd, err := gt.RunNodeErr(gt.Context(), UserPasswdData(input.Static("testuser")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}

	if !passwd.Exists {
		t.Error("Exists = false, want true")
	}
	if passwd.Name != "testuser" {
		t.Errorf("Name = %q, want testuser", passwd.Name)
	}
	if passwd.Password != "x" {
		t.Errorf("Password = %q, want x", passwd.Password)
	}
	if passwd.UID != "1000" || passwd.GID != "1000" {
		t.Errorf("uid:gid = %q:%q, want 1000:1000", passwd.UID, passwd.GID)
	}
	if passwd.GECOS != "Test User" {
		t.Errorf("GECOS = %q, want Test User", passwd.GECOS)
	}
	if passwd.Home != "/home/testuser" {
		t.Errorf("Home = %q, want /home/testuser", passwd.Home)
	}
	if passwd.Shell != "/bin/bash" {
		t.Errorf("Shell = %q, want /bin/bash", passwd.Shell)
	}

	gt.Remote().AssertCommands(gt.Command("getent", "passwd", "testuser"))
}

func TestUserPasswdDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	passwd, err := gt.RunNodeErr(gt.Context(), UserPasswdData(input.Static("missing")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if passwd.Exists {
		t.Error("Exists = true, want false")
	}
}

func TestUserPasswdDataMalformed(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "not-a-passwd-entry\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserPasswdData(input.Static("testuser"))); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestUserSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser:x:1000:1000::/home/testuser:/bin/bash"}
	})
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser sudo\n"}
	})

	if !gt.RunNode(gt.Context(), UserSatisfiedData(input.Static("testuser"), input.Static("/bin/bash"), input.Static("/home/testuser"), input.List("sudo"))) {
		t.Error("UserSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(
		gt.Command("getent", "passwd", "testuser"),
		gt.Command("id", "-nG", "testuser"),
	)
}

func TestUserSatisfiedDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if gt.RunNode(gt.Context(), UserSatisfiedData(input.Static("missing"), input.Static("/bin/bash"), input.Static(""), nil)) {
		t.Error("UserSatisfiedData() = true, want false")
	}
}

func TestUserSatisfiedDataDrift(t *testing.T) {
	tests := []struct {
		name   string
		passwd string
		groups string
		shell  string
		home   string
		want   []string
	}{
		{"shell", "testuser:x:1000:1000::/home/testuser:/bin/sh", "", "/bin/bash", "", nil},
		{"home", "testuser:x:1000:1000::/home/other:/bin/bash", "", "", "/home/testuser", nil},
		{"groups", "testuser:x:1000:1000::/home/testuser:/bin/bash", "testuser", "", "", []string{"sudo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.passwd}
			})
			gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.groups + "\n"}
			})

			if gt.RunNode(gt.Context(), UserSatisfiedData(input.Static("testuser"), input.Static(tt.shell), input.Static(tt.home), input.Static(tt.want))) {
				t.Error("UserSatisfiedData() = true, want false")
			}
		})
	}
}

func TestUserGroupData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser\n"}
	})

	group, err := gt.RunNodeErr(gt.Context(), UserGroupData(input.Static("testuser")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if group != "testuser" {
		t.Errorf("UserGroupData() = %q, want testuser", group)
	}

	gt.Remote().AssertCommands(gt.Command("id", "-gn", "testuser"))
}

func TestUserGroupsData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "testuser sudo docker\n"}
	})

	groups, err := gt.RunNodeErr(gt.Context(), UserGroupsData(input.Static("testuser")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}

	want := []string{"testuser", "sudo", "docker"}
	if !slices.Equal(groups, want) {
		t.Errorf("UserGroupsData() = %v, want %v", groups, want)
	}

	gt.Remote().AssertCommands(gt.Command("id", "-nG", "testuser"))
}

func TestUserGroupDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("id", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserGroupData(input.Static("missing"))); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestUserPasswdDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("getent", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserPasswdData(input.Static("missing"))); err == nil {
		t.Error("UserPasswdData() error = nil, want error")
	}
}

func TestUserPasswdDataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, UserPasswdData(input.Static("testuser"))); !errors.Is(err, sentinel) {
		t.Errorf("UserPasswdData() error = %v, want %v", err, sentinel)
	}
}
