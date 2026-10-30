package command

import (
	"slices"
	"testing"
)

func TestCommandHasArg(t *testing.T) {
	cmd := Command{Args: []string{"compose", "-f", "compose.yaml"}}

	if !cmd.HasArg("-f") {
		t.Error("HasArg(-f) = false, want true")
	}
	if cmd.HasArg("up") {
		t.Error("HasArg(up) = true, want false")
	}
}

func TestSudoEqual(t *testing.T) {
	var nilSudo *Sudo

	if !nilSudo.Equal(nil) {
		t.Error("Equal(nil, nil) = false, want true")
	}
	if nilSudo.Equal(&Sudo{}) {
		t.Error("Equal(nil, non-nil) = true, want false")
	}

	sudo := &Sudo{User: "root", Flags: []string{"-n"}}
	if !sudo.Equal(&Sudo{User: "root", Flags: []string{"-n"}}) {
		t.Error("Equal() = false, want true")
	}
	if sudo.Equal(nil) {
		t.Error("Equal(non-nil, nil) = true, want false")
	}
	if sudo.Equal(&Sudo{User: "deploy", Flags: []string{"-n"}}) {
		t.Error("Equal() = true with a different user, want false")
	}
	if sudo.Equal(&Sudo{User: "root", Flags: []string{"-E"}}) {
		t.Error("Equal() = true with different flags, want false")
	}
}

func TestSudoPrefix(t *testing.T) {
	var nilSudo *Sudo

	if got := nilSudo.Prefix(); got != nil {
		t.Errorf("nil Prefix() = %v, want nil", got)
	}

	if got, want := (&Sudo{}).Prefix(), []string{"sudo"}; !slices.Equal(got, want) {
		t.Errorf("root Prefix() = %v, want %v", got, want)
	}

	sudo := &Sudo{User: "deploy", Flags: []string{"-n", "-E"}}
	if got, want := sudo.Prefix(), []string{"sudo", "-n", "-E", "-u", "deploy"}; !slices.Equal(got, want) {
		t.Errorf("Prefix() = %v, want %v", got, want)
	}
}

func TestSudoWrapCommand(t *testing.T) {
	var nilSudo *Sudo
	cmd := []string{"echo", "hi"}

	if got := nilSudo.WrapCommand(cmd); !slices.Equal(got, cmd) {
		t.Errorf("nil WrapCommand() = %v, want %v", got, cmd)
	}

	sudo := &Sudo{User: "deploy", Flags: []string{"-n"}}
	want := []string{"sudo", "-n", "-u", "deploy", "echo", "hi"}
	if got := sudo.WrapCommand(cmd); !slices.Equal(got, want) {
		t.Errorf("WrapCommand() = %v, want %v", got, want)
	}
}

func TestCommandEqual(t *testing.T) {
	base := Command{
		Name: "docker",
		Args: []string{"compose", "up"},
		Dir:  "/srv",
		Sudo: &Sudo{User: "root", Flags: []string{"-n"}},
	}

	same := Command{
		Name: "docker",
		Args: []string{"compose", "up"},
		Dir:  "/srv",
		Sudo: &Sudo{User: "root", Flags: []string{"-n"}},
	}

	if !base.Equal(same) {
		t.Error("Equal() = false, want true")
	}
	if !same.Equal(base) {
		t.Error("reverse Equal() = false, want true")
	}

	tests := []struct {
		name string
		cmd  Command
	}{
		{"name", Command{Name: "podman", Args: base.Args, Dir: base.Dir, Sudo: base.Sudo}},
		{"args", Command{Name: base.Name, Args: []string{"compose", "down"}, Dir: base.Dir, Sudo: base.Sudo}},
		{"dir", Command{Name: base.Name, Args: base.Args, Dir: "/other", Sudo: base.Sudo}},
		{"sudo nil", Command{Name: base.Name, Args: base.Args, Dir: base.Dir}},
		{"sudo user", Command{Name: base.Name, Args: base.Args, Dir: base.Dir, Sudo: &Sudo{User: "deploy", Flags: []string{"-n"}}}},
		{"sudo flags", Command{Name: base.Name, Args: base.Args, Dir: base.Dir, Sudo: &Sudo{User: "root"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if base.Equal(tt.cmd) {
				t.Error("Equal() = true, want false")
			}
		})
	}

	noSudo := Command{Name: "echo", Args: []string{"hi"}, Dir: "/"}
	noSudoCopy := Command{Name: "echo", Args: []string{"hi"}, Dir: "/"}
	if !noSudo.Equal(noSudoCopy) {
		t.Error("Equal() without sudo = false, want true")
	}
}

func TestCommandEqualExecEnv(t *testing.T) {
	a := Command{Name: "cmd", Args: []string{"a"}, Dir: "/", Env: []string{"A=1"}}
	b := Command{Name: "cmd", Args: []string{"a"}, Dir: "/", Env: []string{"A=1"}}

	if !a.Equal(b) {
		t.Error("Equal() = false, want true")
	}

	c := Command{Name: "cmd", Args: []string{"a"}, Dir: "/", Env: []string{"A=2"}}
	if a.Equal(c) {
		t.Error("Equal() = true with a different exec env, want false")
	}

	d := Command{Name: "cmd", Args: []string{"a"}, Dir: "/"}
	if a.Equal(d) {
		t.Error("Equal() = true with no exec env, want false")
	}
}
