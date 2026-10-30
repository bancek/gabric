package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bancek/gabric"
)

var Build = gabric.Task("build", func(ctx context.Context) {
	gabric.Local(ctx).Exec("go", "build", "./...").Run()
})

var CheckGofmt = gabric.Task("check-gofmt", func(ctx context.Context) {
	out := gabric.Local(ctx).Exec("gofmt", "-l", ".").CaptureStdout().Run().StdoutStr()

	if strings.TrimSpace(out) != "" {
		panic("gofmt found unformatted files:\n" + out)
	}
})

var Vet = gabric.Task("vet", func(ctx context.Context) {
	gabric.Local(ctx).Exec("go", "vet", "./...").Run()
})

var Test = gabric.Task("test", func(ctx context.Context) {
	gabric.Local(ctx).Exec("go", "test", "-race", "./...").Run()
})

var TestWindows = gabric.Task("test-windows", func(ctx context.Context) {
	if runtime.GOOS == "windows" {
		gabric.Local(ctx).Exec("go", "test", "./gabrictest").Run()
		return
	}

	dir, err := os.MkdirTemp("", "gabric-test-windows-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "gabrictest.test.exe")

	gabric.Local(ctx).
		Exec("go", "test", "-c", "-o", bin, "./gabrictest").
		WithEnv(gabric.Env("GOOS", "windows"), gabric.Env("GOARCH", runtime.GOARCH)).
		Run()

	gabric.Local(ctx).
		Exec("docker", "build", "-f", "internal/dev/Dockerfile.wine", "-t", "gabric-wine", ".").
		Run()

	gabric.Local(ctx).Exec(
		"docker", "run", "--rm",
		"-e", "WINEDEBUG=-all",
		"-e", "XDG_RUNTIME_DIR=/tmp",
		"-v", dir+":/tests",
		"gabric-wine",
		"wine", `Z:\tests\gabrictest.test.exe`, "-test.v",
	).Run()
})

var E2ERemote = gabric.Task("e2e-remote", func(ctx context.Context) {
	gabric.Local(ctx).
		Exec("go", "test", "./internal/gabricremote2e", "-count=1").
		WithEnv(gabric.Env("GABRIC_E2E", "1")).
		Run()
})

var E2ESSH = gabric.Task("e2e-ssh", func(ctx context.Context) {
	gabric.Local(ctx).
		Exec("go", "test", "./internal/gabricsshe2e", "-count=1").
		WithEnv(gabric.Env("GABRIC_E2E", "1")).
		Run()
})

var E2EModules = gabric.Task("e2e-modules", func(ctx context.Context) {
	runModuleE2E(ctx, false)
})

var E2EModulesHeavy = gabric.Task("e2e-modules-heavy", func(ctx context.Context) {
	runModuleE2E(ctx, true)
})

func runModuleE2E(ctx context.Context, heavy bool) {
	run := `^Test.*E2E$`
	if arg := gabric.GetCurrentTaskArg(ctx); arg != "" {
		run = arg
	}

	timeout := "30m"
	env := []gabric.EnvVar{gabric.Env("GABRIC_E2E", "1")}

	if heavy {
		timeout = "60m"
		env = append(env, gabric.Env("GABRIC_E2E_HEAVY", "1"))
	}

	gabric.Local(ctx).
		Exec("go", "test", "-p", "1", "-run", run, "-count=1", "-v", "-timeout", timeout,
			"./reconcile/modules/...").
		WithEnv(env...).
		Run()
}

var E2ETest = gabric.Task("e2e-test", func(ctx context.Context) {
	E2ERemote(ctx)
	E2ESSH(ctx)
})

var E2E = gabric.Task("e2e", func(ctx context.Context) {
	E2ETest(ctx)
	E2EAPIDemo(ctx)
})

var CI = gabric.Task("ci", func(ctx context.Context) {
	gabric.Parallel(ctx, CheckGofmt, Vet, Test)
})

var Gofmt = gabric.Task("gofmt", func(ctx context.Context) {
	gabric.Local(ctx).Exec("gofmt", "-w", ".").Run()
})

var Prettier = gabric.Task("prettier", func(ctx context.Context) {
	gabric.Local(ctx).Exec("npx", "prettier", "-w", "AGENTS.md", "README.md", "./docs").Run()
})

var Format = gabric.Task("format", func(ctx context.Context) {
	Gofmt(ctx)
	Prettier(ctx)
})

func main() {
	gabric.Main()
}
