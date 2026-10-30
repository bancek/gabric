package gabrictest

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"testing"

	"github.com/bancek/gabric"
)

func TestRemoteFileOperations(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	err := gabric.Remote(ctx).Put("/srv/app/config").
		String("hello").
		Mode(0o600).
		User("root").
		Group("root").
		EnsureParent().
		RunErr()
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	g.Remote().AssertPutCalled(gabric.PutRequest{
		Path:         "/srv/app/config",
		Content:      bytes.NewBufferString("hello"),
		Mode:         0o600,
		User:         "root",
		Group:        "root",
		EnsureParent: true,
	})

	content, err := g.Remote().ReadFile("/srv/app/config")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}

	info, err := g.Remote().Stat("/srv/app/config")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	if got := gabric.Remote(ctx).Get("/srv/app/config").String(); got != "hello" {
		t.Errorf("Get() = %q, want %q", got, "hello")
	}
	g.Remote().AssertGetCalled(gabric.GetRequest{Path: "/srv/app/config"})

	if !gabric.Remote(ctx).Exists("/srv/app/config").File().Run() {
		t.Error("Exists().File() = false, want true")
	}
	g.Remote().AssertExistsCalled(gabric.ExistsRequest{Path: "/srv/app/config", Kind: gabric.FileKindFile})

	if err := gabric.Remote(ctx).Mkdir("/srv/cache").Recursive().Mode(0o700).RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	g.Remote().AssertMkdirCalled(gabric.MkdirRequest{
		Path:      "/srv/cache",
		Recursive: true,
		Mode:      0o700,
	})

	info, err = g.Remote().Stat("/srv/cache")
	if err != nil {
		t.Fatalf("Stat(cache) error: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache is not a directory")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Errorf("cache mode = %v, want %v", got, want)
	}
}

func TestLocalFileOperations(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	if err := gabric.Local(ctx).Mkdir("/srv/app").Recursive().RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := gabric.Local(ctx).Put("/srv/app/config").String("local").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if got := gabric.Local(ctx).Get("/srv/app/config").String(); got != "local" {
		t.Errorf("Get() = %q, want %q", got, "local")
	}
	if !gabric.Local(ctx).Exists("/srv/app").Dir().Run() {
		t.Error("Exists().Dir() = false, want true")
	}

	g.Local().AssertMkdirCalled(gabric.MkdirRequest{Path: "/srv/app", Recursive: true})
	g.Local().AssertPutCalled(gabric.PutRequest{Path: "/srv/app/config", Content: bytes.NewBufferString("local")})
	g.Local().AssertGetCalled(gabric.GetRequest{Path: "/srv/app/config"})
	g.Local().AssertExistsCalled(gabric.ExistsRequest{Path: "/srv/app", Kind: gabric.FileKindDir})
}

func TestRemoveCalls(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Remote().WriteFile("/srv/app/file", []byte("x"), 0o644)

	if err := gabric.Remote(ctx).Remove("/srv/app/file").RunErr(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	g.Remote().AssertRemoveCalled(gabric.RemoveRequest{Path: "/srv/app/file"})

	if _, err := g.Remote().Stat("/srv/app/file"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want fs.ErrNotExist", err)
	}

	g.Remote().MkdirAll("/srv/tree/sub", 0o755)

	err := gabric.Remote(ctx).Remove("/srv/tree").RunErr()
	if err != nil {
		// expected
	} else {
		t.Error("Remove(non-empty dir) = nil, want error")
	}

	if err := gabric.Remote(ctx).Remove("/srv/tree").Recursive().Force().RunErr(); err != nil {
		t.Fatalf("Remove().Recursive().Force() error: %v", err)
	}

	g.Remote().AssertRemoveCalled(gabric.RemoveRequest{Path: "/srv/tree", Recursive: true, Force: true})

	if err := gabric.Remote(ctx).Remove("/srv/missing").Force().RunErr(); err != nil {
		t.Errorf("Remove(missing).Force() error = %v, want nil", err)
	}

	g.Local().WriteFile("/srv/app/file", []byte("x"), 0o644)

	if err := gabric.Local(ctx).Remove("/srv/app/file").RunErr(); err != nil {
		t.Fatalf("Local Remove() error: %v", err)
	}

	g.Local().AssertRemoveCalled(gabric.RemoveRequest{Path: "/srv/app/file"})

	if _, err := g.Local().Stat("/srv/app/file"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() error = %v, want fs.ErrNotExist", err)
	}
}

func TestStatCalls(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Remote().WriteFile("/srv/file", []byte("x"), 0o640)
	g.Remote().SetOwner("/srv/file", "testuser", "staff")
	g.Local().WriteFile("/srv/file", []byte("x"), 0o640)
	g.Local().SetOwner("/srv/file", "testuser", "staff")

	if _, err := gabric.Remote(ctx).Stat("/srv/file").RunErr(); err != nil {
		t.Fatalf("Remote.Stat() error: %v", err)
	}

	stat, err := gabric.Local(ctx).Stat("/srv/file").RunErr()
	if err != nil {
		t.Fatalf("Local.Stat() error: %v", err)
	}

	if got, want := stat.User, "testuser"; got != want {
		t.Errorf("User = %q, want %q", got, want)
	}

	g.Remote().AssertStatCalled(gabric.StatRequest{Path: "/srv/file"})
	g.Local().AssertStatCalled(gabric.StatRequest{Path: "/srv/file"})
}

func TestAssertCommandsAndCalls(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Remote().AssertCommands()
	g.Remote().AssertCalls()

	g.Remote().Handle("one", func(Command) Result { return Result{} })
	g.Remote().Handle("two", func(Command) Result { return Result{} })

	if err := gabric.Remote(ctx).Exec("one", "a").RunErr(); err != nil {
		t.Fatalf("Exec(one) error: %v", err)
	}
	if err := gabric.Remote(ctx).Exec("two", "b").RunErr(); err != nil {
		t.Fatalf("Exec(two) error: %v", err)
	}

	g.Remote().AssertCommands(
		Command{Name: "one", Args: []string{"a"}, Dir: "/"},
		Command{Name: "two", Args: []string{"b"}, Dir: "/"},
	)

	if got := g.Remote().Commands(); len(got) != 2 {
		t.Errorf("Commands() length = %d, want 2", len(got))
	}

	if err := gabric.Remote(ctx).Mkdir("/srv").Recursive().RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := gabric.Remote(ctx).Put("/srv/x").String("x").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	wantCalls := []Call{
		{Mkdir: &gabric.MkdirRequest{Path: "/srv", Recursive: true}},
		{Put: &gabric.PutRequest{Path: "/srv/x", Content: bytes.NewBufferString("x")}},
	}

	g.Remote().AssertCalls(wantCalls...)

	g.Remote().ResetCommands()
	g.Remote().AssertCommands()
	g.Remote().AssertCalls(wantCalls...)

	g.Remote().ResetCalls()
	g.Remote().AssertCalls()
}

func TestCallsPreserveOrder(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	if err := gabric.Remote(ctx).Mkdir("/srv/app").Recursive().RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := gabric.Remote(ctx).Put("/srv/app/config").String("x").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if got := gabric.Remote(ctx).Get("/srv/app/config").String(); got != "x" {
		t.Errorf("Get() = %q, want %q", got, "x")
	}
	if !gabric.Remote(ctx).Exists("/srv/app/config").File().Run() {
		t.Error("Exists() = false, want true")
	}

	want := []Call{
		{Mkdir: &gabric.MkdirRequest{Path: "/srv/app", Recursive: true}},
		{Put: &gabric.PutRequest{Path: "/srv/app/config", Content: bytes.NewBufferString("x")}},
		{Get: &gabric.GetRequest{Path: "/srv/app/config"}},
		{Exists: &gabric.ExistsRequest{Path: "/srv/app/config", Kind: gabric.FileKindFile}},
	}

	if got := g.Remote().Calls(); !slices.EqualFunc(got, want, Call.Equal) {
		t.Errorf("Calls() = %v, want %v", got, want)
	}
}

func TestAssertPutCalledRepeated(t *testing.T) {
	g := New(t)

	if err := gabric.Remote(g.Context()).Put("/srv/config").EnsureParent().String("x").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	want := gabric.PutRequest{
		Path:         "/srv/config",
		Content:      bytes.NewBufferString("x"),
		EnsureParent: true,
	}

	g.Remote().AssertPutCalled(want)
	g.Remote().AssertPutCalled(want)
}

func TestCallsReturnsIndependentCopies(t *testing.T) {
	g := New(t)

	if err := gabric.Remote(g.Context()).Put("/srv/config").EnsureParent().String("x").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	first := g.Remote().Calls()
	first[0].Put.Path = "/changed"
	first[0].Put.Content.(*bytes.Buffer).WriteString("changed")

	want := []Call{{
		Put: &gabric.PutRequest{
			Path:         "/srv/config",
			Content:      bytes.NewBufferString("x"),
			EnsureParent: true,
		},
	}}

	if got := g.Remote().Calls(); !slices.EqualFunc(got, want, Call.Equal) {
		t.Errorf("Calls() changed after mutating a snapshot: got %v, want %v", got, want)
	}
}

func TestPutMissingParent(t *testing.T) {
	g := New(t)

	err := gabric.Remote(g.Context()).Put("/srv/missing/file").String("x").RunErr()
	if err != nil {
		return
	}

	t.Error("Put(missing parent) = nil, want error")
}

func TestPutPreservesExistingMode(t *testing.T) {
	g := New(t)

	g.Remote().WriteFile("/srv/config", []byte("old"), 0o600)

	if err := gabric.Remote(g.Context()).Put("/srv/config").String("new").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	info, err := g.Remote().Stat("/srv/config")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestMkdirExistingMode(t *testing.T) {
	g := New(t)

	g.Remote().MkdirAll("/srv/app", 0o755)

	if err := gabric.Remote(g.Context()).Mkdir("/srv/app").Mode(0o700).RunErr(); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	info, err := g.Remote().Stat("/srv/app")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestExecHandler(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Remote().Handle("docker", func(cmd Command) Result {
		if cmd.HasArg("--dry-run") {
			return Result{Stderr: "drift\n"}
		}

		return Result{Stdout: "ok\n"}
	})

	out := gabric.Remote(ctx).Exec("docker", "compose", "up", "-d").Capture().Run()
	if got := out.StdoutStr(); got != "ok\n" {
		t.Errorf("StdoutStr() = %q, want %q", got, "ok\n")
	}

	cmd := g.Remote().AssertExecuted("docker", "compose", "up", "-d")
	if cmd.Dir != "/" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/")
	}
	if cmd.Sudo != nil {
		t.Errorf("Sudo = %+v, want nil", cmd.Sudo)
	}

	errOut := gabric.Remote(ctx).Exec("docker", "compose", "--dry-run").Capture().Run()
	if got := errOut.StderrStr(); got != "drift\n" {
		t.Errorf("StderrStr() = %q, want %q", got, "drift\n")
	}

	nested := gabric.Remote(ctx).Exec("sh", "-c", "echo nested").CaptureStdout().Run()
	if got := nested.StdoutStr(); got != "nested\n" {
		t.Errorf("nested StdoutStr() = %q, want %q", got, "nested\n")
	}
	g.Remote().AssertExecuted("echo", "nested")

	g.Remote().AssertNotExecuted("docker", "down")
}

func TestLocalExec(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Local().Handle("custom", func(Command) Result { return Result{Stdout: "local\n"} })

	out := gabric.Local(ctx).Exec("custom").CaptureStdout().Run()
	if got := out.StdoutStr(); got != "local\n" {
		t.Errorf("StdoutStr() = %q, want %q", got, "local\n")
	}

	cmd := g.Local().AssertExecuted("custom")
	if cmd.Dir != "/" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/")
	}
}

func TestExecWorkingDirAndSudo(t *testing.T) {
	g := New(t)
	g.Remote().MkdirAll("/srv/app", 0o755)

	ctx := g.Context()
	g.Remote().Handle("custom", func(Command) Result { return Result{} })

	gabric.Remote(ctx).WithWorkingDir("/srv/app", func(ctx context.Context) {
		ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{User: "app", Flags: []string{"-n"}})

		if err := gabric.Remote(ctx).Exec("custom").RunErr(); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	cmd := g.Remote().AssertExecuted("custom")
	if cmd.Dir != "/srv/app" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/srv/app")
	}
	if cmd.Sudo == nil {
		t.Fatal("Sudo = nil, want non-nil")
	}
	if cmd.Sudo.User != "app" {
		t.Errorf("Sudo.User = %q, want app", cmd.Sudo.User)
	}
	if len(cmd.Sudo.Flags) != 1 || cmd.Sudo.Flags[0] != "-n" {
		t.Errorf("Sudo.Flags = %v, want [-n]", cmd.Sudo.Flags)
	}
}

func TestListCall(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		g := New(t)
		g.Local().MkdirAll("/srv/app/sub", 0o755)
		g.Local().WriteFile("/srv/app/config", []byte("x"), 0o644)

		entries := gabric.Local(g.Context()).List("/srv/app")

		want := []gabric.ListEntry{
			{Name: "config"},
			{Name: "sub", IsDir: true},
		}
		if !slices.Equal(entries, want) {
			t.Errorf("List() = %+v, want %+v", entries, want)
		}

		g.Local().AssertListCalled(gabric.ListRequest{Path: "/srv/app"})
		g.Local().AssertCalls(Call{List: &gabric.ListRequest{Path: "/srv/app"}})
	})

	t.Run("remote", func(t *testing.T) {
		g := New(t)
		g.Remote().MkdirAll("/srv/app/sub", 0o755)
		g.Remote().WriteFile("/srv/app/config", []byte("x"), 0o644)

		entries := gabric.Remote(g.Context()).List("/srv/app")

		want := []gabric.ListEntry{
			{Name: "config"},
			{Name: "sub", IsDir: true},
		}
		if !slices.Equal(entries, want) {
			t.Errorf("List() = %+v, want %+v", entries, want)
		}

		g.Remote().AssertListCalled(gabric.ListRequest{Path: "/srv/app"})
		g.Remote().AssertCalls(Call{List: &gabric.ListRequest{Path: "/srv/app"}})
	})
}

func TestListErr(t *testing.T) {
	g := New(t)

	if _, err := gabric.Remote(g.Context()).ListErr("/srv/missing"); err == nil {
		t.Error("ListErr() error = nil, want error")
	}

	g.Remote().AssertListCalled(gabric.ListRequest{Path: "/srv/missing"})
}

func TestRsyncHandler(t *testing.T) {
	g := New(t)
	g.Remote().HandleRsync(func(gabric.RsyncRequest) Result {
		return Result{Stdout: ">file\n"}
	})

	out := gabric.Remote(g.Context()).Rsync().
		Local("./app").
		Remote("/home/app").
		Upload().
		CaptureStdout().
		Run()

	if out.StdoutStr() != ">file\n" {
		t.Errorf("StdoutStr() = %q, want %q", out.StdoutStr(), ">file\n")
	}

	g.Remote().AssertRsyncCalled(gabric.RsyncRequest{
		LocalPath:  "./app",
		RemotePath: "/home/app",
		Direction:  gabric.RsyncDirectionUpload,
		Flags:      []string{"-pthrvz"},
	})
}

func TestRsyncHandlerError(t *testing.T) {
	g := New(t)
	g.Remote().HandleRsync(func(gabric.RsyncRequest) Result {
		return Result{ExitCode: 2}
	})

	err := gabric.Remote(g.Context()).Rsync().
		Local("./app").
		Remote("/home/app").
		Upload().
		RunErr()
	if err != nil {
		return
	}

	t.Error("RunErr() error = nil, want error")
}

func TestRsyncCall(t *testing.T) {
	g := New(t)
	ctx := gabric.NewRemoteSudoContext(g.Context(), &gabric.Sudo{})

	err := gabric.Remote(ctx).Rsync().
		Local("/srv/app").
		Remote("/srv/app").
		Upload().
		Delete().
		Exclude("**/.env", ".state/**").
		AddFlags("--info").
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	want := gabric.RsyncRequest{
		LocalPath:  "/srv/app",
		RemotePath: "/srv/app",
		Direction:  gabric.RsyncDirectionUpload,
		Delete:     true,
		Excludes:   []string{"**/.env", ".state/**"},
		Flags:      []string{"-pthrvz", "--info"},
		Sudo:       &gabric.Sudo{},
	}

	g.Remote().AssertRsyncCalled(want)
	g.Remote().AssertCalls(Call{Rsync: &want})
	g.Local().AssertCalls()
	g.Remote().AssertCommands()
	g.Local().AssertCommands()
}

func TestRsyncCallDownload(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	err := gabric.Remote(ctx).Rsync().
		Local("/srv/app").
		Remote("/srv/app").
		Download().
		SetFlags("-a", "--checksum").
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	want := gabric.RsyncRequest{
		LocalPath:  "/srv/app",
		RemotePath: "/srv/app",
		Direction:  gabric.RsyncDirectionDownload,
		Flags:      []string{"-a", "--checksum"},
	}

	g.Remote().AssertRsyncCalled(want)
	g.Remote().AssertCalls(Call{Rsync: &want})
	g.Local().AssertCalls()
}

func TestRsyncCallValidation(t *testing.T) {
	g := New(t)

	tests := []struct {
		name string
		ops  *gabric.RsyncOps
	}{
		{
			name: "missing direction",
			ops:  gabric.Remote(g.Context()).Rsync().Local("./app").Remote("/home/app"),
		},
		{
			name: "missing local path",
			ops:  gabric.Remote(g.Context()).Rsync().Remote("/home/app").Upload(),
		},
		{
			name: "missing remote path",
			ops:  gabric.Remote(g.Context()).Rsync().Local("./app").Upload(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ops.RunErr(); err == nil {
				t.Fatal("RunErr() = nil, want error")
			}
		})
	}

	g.Remote().AssertCalls()
	g.Local().AssertCalls()
}

func TestRsyncCallReturnsIndependentCopy(t *testing.T) {
	g := New(t)

	err := gabric.Remote(g.Context()).Rsync().
		Local("./app").
		Remote("/home/app").
		Upload().
		Exclude("**/.env").
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	calls := g.Remote().Calls()
	calls[0].Rsync.Excludes[0] = "changed"
	calls[0].Rsync.Flags[0] = "changed"

	g.Remote().AssertRsyncCalled(gabric.RsyncRequest{
		LocalPath:  "./app",
		RemotePath: "/home/app",
		Direction:  gabric.RsyncDirectionUpload,
		Excludes:   []string{"**/.env"},
		Flags:      []string{"-pthrvz"},
	})
}

func TestExecFailure(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	g.Remote().Handle("fail", func(Command) Result { return Result{ExitCode: 3} })

	if err := gabric.Remote(ctx).Exec("fail").RunErr(); err == nil {
		t.Error("RunErr() = nil, want error")
	}

	if err := gabric.Remote(ctx).Exec("not-a-command").Capture().RunErr(); err == nil {
		t.Error("RunErr(unknown) = nil, want error")
	}
}

func TestLocalExecExtraFilesUnsupported(t *testing.T) {
	g := New(t)

	tests := []struct {
		name string
		req  gabric.ExecRequest
	}{
		{
			name: "extra files",
			req:  gabric.ExecRequest{Cmd: []string{"rsync"}, ExtraFiles: []*os.File{nil}},
		},
		{
			name: "after start",
			req:  gabric.ExecRequest{Cmd: []string{"rsync"}, AfterStart: func() {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (localBackend{side: g.local}).Exec(g.Context(), tt.req)
			if !errors.Is(err, errLocalExecUnsupported) {
				t.Errorf("Exec() error = %v, want errLocalExecUnsupported", err)
			}
		})
	}
}

func TestRemoteExecEnv(t *testing.T) {
	g := New(t)
	ctx := gabric.AppendRemoteExecEnvContext(g.Context(), gabric.Env("GABRIC_TEST_ENV", "value"))

	var handlerEnv []string

	g.Remote().Handle("envcheck", func(cmd Command) Result {
		handlerEnv = cmd.Env
		return Result{Stdout: "ok"}
	})

	out := gabric.TargetOps(ctx).Exec("envcheck").CaptureStdout().Run().StdoutStr()
	if want := "ok"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if want := []string{"GABRIC_TEST_ENV=value"}; !slices.Equal(handlerEnv, want) {
		t.Errorf("handler env = %v, want %v", handlerEnv, want)
	}

	cmd := g.Remote().AssertExecuted("envcheck")
	if want := []string{"GABRIC_TEST_ENV=value"}; !slices.Equal(cmd.Env, want) {
		t.Errorf("recorded env = %v, want %v", cmd.Env, want)
	}
}

func TestLocalExecEnv(t *testing.T) {
	g := New(t)
	ctx := gabric.NewTargetContext(g.Context(), gabric.LocalTarget)
	ctx = gabric.AppendLocalExecEnvContext(ctx, gabric.Env("GABRIC_TEST_ENV", "local"))

	var handlerEnv []string

	g.Local().Handle("envcheck", func(cmd Command) Result {
		handlerEnv = cmd.Env
		return Result{}
	})

	if err := gabric.TargetOps(ctx).Exec("envcheck").RunErr(); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if want := []string{"GABRIC_TEST_ENV=local"}; !slices.Equal(handlerEnv, want) {
		t.Errorf("handler env = %v, want %v", handlerEnv, want)
	}
}

func TestTargetOpsExecEnv(t *testing.T) {
	g := New(t)
	ctx := gabric.NewTargetOpsExecEnvContext(g.Context(), gabric.Env("GABRIC_TEST_ENV", "overlay"))

	var handlerEnv []string

	g.Remote().Handle("envcheck", func(cmd Command) Result {
		handlerEnv = cmd.Env
		return Result{}
	})

	if err := gabric.TargetOps(ctx).Exec("envcheck").RunErr(); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if want := []string{"GABRIC_TEST_ENV=overlay"}; !slices.Equal(handlerEnv, want) {
		t.Errorf("handler env = %v, want %v", handlerEnv, want)
	}
}

func TestExecEnvPerCommand(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	var handlerEnv []string

	g.Remote().Handle("envcheck", func(cmd Command) Result {
		handlerEnv = cmd.Env
		return Result{}
	})

	if err := gabric.TargetOps(ctx).Exec("envcheck").WithEnv(gabric.Env("GABRIC_TEST_ENV", "command")).RunErr(); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if want := []string{"GABRIC_TEST_ENV=command"}; !slices.Equal(handlerEnv, want) {
		t.Errorf("handler env = %v, want %v", handlerEnv, want)
	}
}

func TestExecEnvNestedShell(t *testing.T) {
	g := New(t)
	ctx := gabric.AppendRemoteExecEnvContext(g.Context(), gabric.Env("GABRIC_TEST_ENV", "value"))

	out := gabric.TargetOps(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_ENV").
		CaptureStdout().
		Run().
		StdoutStr()

	if want := "value\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestAssertCommandsEnv(t *testing.T) {
	g := New(t)
	ctx := gabric.AppendRemoteExecEnvContext(g.Context(), gabric.Env("GABRIC_TEST_ENV", "value"))

	g.Remote().Handle("envcheck", func(Command) Result {
		return Result{}
	})

	if err := gabric.TargetOps(ctx).Exec("envcheck").RunErr(); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}

	want := g.Command("envcheck")
	want.Env = []string{"GABRIC_TEST_ENV=value"}
	g.Remote().AssertCommands(want)
}
