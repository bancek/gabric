package gabrice2e

import (
	"slices"
	"testing"
)

func TestContainerRunArgs(t *testing.T) {
	net := &Network{name: "test-net"}

	cfg := applyContainerOptions([]ContainerOption{
		WithContainerName("app"),
		WithContainerPrivileged(),
		WithContainerCgroupnsHost(),
		WithContainerUser("app"),
		WithContainerEnv("FOO=bar", "BAZ=qux"),
		WithContainerVolume("/host", "/container", true),
		WithContainerVolume("/host2", "/container2", false),
		WithContainerAddHost("host.docker.internal", "host-gateway"),
		WithContainerStopSignal("SIGRTMIN+3"),
		WithContainerSecurityOpt("seccomp=unconfined"),
		WithContainerTmpfs("/run"),
		WithContainerTmpfs("/var/tmp"),
		WithContainerPort(22),
		WithContainerPort(80),
		WithContainerCommand("sh", "-c", "sleep 1"),
		WithContainerNetwork(net),
	})

	got := cfg.runArgs("image:tag")
	want := []string{
		"run", "-d", "--rm",
		"--name", "app",
		"--privileged",
		"--cgroupns=host",
		"--user", "app",
		"-e", "FOO=bar",
		"-e", "BAZ=qux",
		"-v", "/host:/container:ro",
		"-v", "/host2:/container2",
		"--add-host", "host.docker.internal:host-gateway",
		"--stop-signal", "SIGRTMIN+3",
		"--security-opt", "seccomp=unconfined",
		"--tmpfs", "/run",
		"--tmpfs", "/var/tmp",
		"--network", "test-net",
		"-p", "127.0.0.1::22",
		"-p", "127.0.0.1::80",
		"image:tag",
		"sh", "-c", "sleep 1",
	}

	if !slices.Equal(got, want) {
		t.Errorf("runArgs() = %v, want %v", got, want)
	}
}

func TestContainerOptionsInvalidPort(t *testing.T) {
	for _, port := range []int{-1, 0, 65536} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithContainerPort(%d) did not panic", port)
				}
			}()

			WithContainerPort(port)
		}()
	}
}

func TestContainerOptionsEmptyTmpfs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithContainerTmpfs(\"\") did not panic")
		}
	}()

	WithContainerTmpfs("")
}

func TestContainerOptionsNilNetwork(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithContainerNetwork(nil) did not panic")
		}
	}()

	WithContainerNetwork(nil)
}

func TestContainerOptionsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"user", func() { WithContainerUser("") }},
		{"env", func() { WithContainerEnv("") }},
		{"volume", func() { WithContainerVolume("", "/container", false) }},
		{"add host", func() { WithContainerAddHost("", "host-gateway") }},
		{"stop signal", func() { WithContainerStopSignal("") }},
		{"security opt", func() { WithContainerSecurityOpt("") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s option did not panic", tt.name)
				}
			}()

			tt.fn()
		})
	}
}
