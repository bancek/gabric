package moduletest

import (
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrice2e"
	"github.com/bancek/gabric/internal/testutils"
)

// Target is a module e2e target with its effective identity and capabilities.
type Target struct {
	*gabrice2e.Test

	// User and Group are the effective target user and primary group.
	User  string
	Group string

	// Capabilities.
	Root       bool
	POSIXModes bool
	Symlinks   bool
	Linux      bool
	Systemd    bool
	Docker     bool
	Apt        bool
	Git        bool
	Rsync      bool
	Curl       bool

	docker *gabrice2e.DockerTest
	dir    string
	join   func(...string) string
	isAbs  func(string) bool
	local  bool
}

// Require skips the test unless GABRIC_E2E is set.
func Require(t testing.TB) {
	t.Helper()

	if os.Getenv("GABRIC_E2E") == "" {
		t.Skip("moduletest: set GABRIC_E2E=1 to run module e2e tests")
	}
}

// RequireHeavy skips the test unless GABRIC_E2E and GABRIC_E2E_HEAVY are set.
func RequireHeavy(t testing.TB) {
	t.Helper()

	Require(t)

	if os.Getenv("GABRIC_E2E_HEAVY") == "" {
		t.Skip("moduletest: set GABRIC_E2E_HEAVY=1 to run heavy module e2e tests")
	}
}

// Docker returns a target backed by a Docker container.
func Docker(t *testing.T, image gabrice2e.Image, opts ...gabrice2e.ContainerOption) *Target {
	t.Helper()

	d := gabrice2e.Docker(t, image, opts...)
	dir := path.Join("/tmp", d.Container().Name())
	target := &Target{
		Test:   d.Test,
		docker: d,
		dir:    dir,
		join:   path.Join,
		isAbs:  path.IsAbs,
	}

	if err := target.Ops().Mkdir(dir).Recursive().RunErr(); err != nil {
		t.Fatalf("moduletest: create target directory %s: %v", dir, err)
	}

	target.init(t)

	return target
}

// Local returns a plain local target.
func Local(t *testing.T) *Target {
	t.Helper()

	test := gabrice2e.Local(t)
	target := &Target{
		Test:  test,
		dir:   gabric.GetLocalWorkingDir(test.Context()),
		join:  filepath.Join,
		isAbs: filepath.IsAbs,
		local: true,
	}
	target.init(t)

	return target
}

// LocalRemote returns a remote-style local target.
func LocalRemote(t *testing.T) *Target {
	t.Helper()

	test := gabrice2e.LocalRemote(t)
	target := &Target{
		Test:  test,
		dir:   gabric.GetRemoteWorkingDir(test.Context()),
		join:  path.Join,
		isAbs: path.IsAbs,
	}
	target.init(t)

	return target
}

// Container returns the Docker container for a Docker target, or nil for Local
// and LocalRemote targets.
func (g *Target) Container() *gabrice2e.Container {
	if g.docker == nil {
		return nil
	}

	return g.docker.Container()
}

// Ops returns the target operations.
func (g *Target) Ops() gabric.Ops {
	return gabric.TargetOps(g.Context())
}

// Path returns name under the target's working directory. Absolute names are
// returned unchanged.
func (g *Target) Path(name string) string {
	if name == "" {
		return g.dir
	}
	if g.isAbs(name) {
		return name
	}

	return g.join(g.dir, name)
}

func (g *Target) init(t *testing.T) {
	t.Helper()

	if g.local {
		g.initLocal(t)
	} else {
		g.initOps(t)
	}

	g.POSIXModes = g.docker != nil || runtime.GOOS != "windows"
	g.Symlinks = g.POSIXModes
	g.Linux = g.docker != nil || runtime.GOOS == "linux"
	g.Systemd = g.pathIsDir("/run/systemd/system")
	g.Apt = g.commandAvailable("apt-get")
	g.Git = g.commandAvailable("git")
	g.Rsync = g.commandAvailable("rsync")
	g.Curl = g.commandAvailable("curl")
	g.Docker = g.commandAvailable("docker")
}

func (g *Target) initLocal(t *testing.T) {
	t.Helper()

	u, err := user.Current()
	if err != nil {
		t.Fatalf("moduletest: local user: %v", err)
	}

	g.User = u.Username
	if group, err := user.LookupGroupId(u.Gid); err == nil {
		g.Group = group.Name
	} else {
		g.Group = u.Gid
	}
	g.Root = os.Geteuid() == 0
}

func (g *Target) initOps(t *testing.T) {
	t.Helper()

	g.User = g.captureID(t, "id", "-un")
	g.Group = g.captureID(t, "id", "-gn")
	g.Root = g.captureID(t, "id", "-u") == "0"
}

func (g *Target) captureID(t *testing.T, name string, args ...string) string {
	t.Helper()

	e := g.Ops().Exec(append([]string{name}, args...)...).CaptureStdout()
	if err := e.RunErr(); err != nil {
		t.Fatalf("moduletest: %s %s: %v", name, strings.Join(args, " "), err)
	}

	return strings.TrimSpace(e.StdoutStr())
}

func (g *Target) pathIsDir(name string) bool {
	exists, err := g.Ops().Exists(name).Dir().RunErr()
	if err != nil {
		return false
	}

	return exists
}

func (g *Target) commandAvailable(name string) bool {
	if g.local {
		_, err := exec.LookPath(name)
		return err == nil
	}

	return g.Ops().Exec("sh", "-c", "command -v "+name).CaptureStdout().RunErr() == nil
}

// ForEachBaseTarget runs fn for the base module targets.
func ForEachBaseTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	Require(t)

	targets := []struct {
		name string
		new  func(*testing.T) *Target
	}{
		{"docker/alpine-root", func(t *testing.T) *Target { return Docker(t, Alpine) }},
		{"docker/alpine-user", func(t *testing.T) *Target { return Docker(t, AlpineUser) }},
		{"docker/ubuntu-root", func(t *testing.T) *Target { return Docker(t, Ubuntu) }},
		{"docker/ubuntu-user", func(t *testing.T) *Target { return Docker(t, UbuntuUser) }},
		{"localremote", LocalRemote},
		{"local", Local},
	}

	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			fn(tc.new(t))
		})
	}
}

// ForEachLinuxTarget runs fn for the Linux account/kernel targets.
func ForEachLinuxTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	Require(t)

	targets := []struct {
		name  string
		image gabrice2e.Image
	}{
		{"docker/alpine-tools", AlpineTools},
		{"docker/ubuntu-tools", UbuntuTools},
	}

	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			fn(Docker(t, tc.image))
		})
	}
}

type toolsTarget struct {
	name  string
	image gabrice2e.Image
	user  string
}

var toolsTargets = []toolsTarget{
	{"docker/alpine-tools-root", AlpineTools, ""},
	{"docker/alpine-tools-user", AlpineTools, "app"},
	{"docker/ubuntu-tools-root", UbuntuTools, ""},
	{"docker/ubuntu-tools-user", UbuntuTools, "app"},
}

// ForEachToolsTarget runs fn for the tools images as root and the app user.
func ForEachToolsTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	Require(t)

	for _, tc := range toolsTargets {
		t.Run(tc.name, func(t *testing.T) {
			fn(Docker(t, tc.image, toolsOptions(tc)...))
		})
	}
}

// ForEachHTTPTarget runs fn for the tools images as root and the app user, with
// a file server on the same Docker network.
func ForEachHTTPTarget(t *testing.T, fn func(*Target, *FileServer)) {
	t.Helper()
	Require(t)

	for _, tc := range toolsTargets {
		t.Run(tc.name, func(t *testing.T) {
			network := gabrice2e.RunNetwork(t)
			opts := append(toolsOptions(tc), gabrice2e.WithContainerNetwork(network))
			g := Docker(t, tc.image, opts...)
			server := NewFileServer(t, network, g)
			fn(g, server)
		})
	}
}

func toolsOptions(tc toolsTarget) []gabrice2e.ContainerOption {
	if tc.user == "" {
		return nil
	}

	return []gabrice2e.ContainerOption{gabrice2e.WithContainerUser(tc.user)}
}

// ForEachUbuntuTarget runs fn for the Ubuntu target.
func ForEachUbuntuTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	Require(t)

	t.Run("docker/ubuntu", func(t *testing.T) {
		fn(Docker(t, Ubuntu))
	})
}

// ForEachAlpineTarget runs fn for the Alpine target.
func ForEachAlpineTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	Require(t)

	t.Run("docker/alpine", func(t *testing.T) {
		fn(Docker(t, Alpine))
	})
}

// ForEachLinuxSysctlTarget runs fn for a privileged Linux target.
func ForEachLinuxSysctlTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	RequireHeavy(t)

	t.Run("docker/ubuntu-tools-privileged", func(t *testing.T) {
		fn(Docker(t, UbuntuTools, gabrice2e.WithContainerPrivileged()))
	})
}

// ForEachDockerTarget runs fn for a Docker-capable target.
func ForEachDockerTarget(t *testing.T, image gabrice2e.Image, fn func(*Target)) {
	t.Helper()
	RequireHeavy(t)

	t.Run("docker/"+image.Name(), func(t *testing.T) {
		g := Docker(t, image,
			gabrice2e.WithContainerPrivileged(),
			gabrice2e.WithContainerEnv("DOCKER_TLS_CERTDIR="),
		)
		waitForDocker(t, g)
		fn(g)
	})
}

// ForEachSystemdTarget runs fn for the systemd target.
func ForEachSystemdTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	RequireHeavy(t)

	t.Run("docker/ubuntu-systemd", func(t *testing.T) {
		g := Docker(t, UbuntuSystemd, systemdContainerOptions()...)
		waitForSystemd(t, g)
		fn(g)
	})
}

// ForEachUfwDockerTarget runs fn for the ufw-docker systemd target.
func ForEachUfwDockerTarget(t *testing.T, fn func(*Target)) {
	t.Helper()
	RequireHeavy(t)

	t.Run("docker/ubuntu-ufwdocker", func(t *testing.T) {
		g := Docker(t, UbuntuUfwDocker, systemdContainerOptions()...)
		waitForSystemd(t, g)
		fn(g)
	})
}

func systemdContainerOptions() []gabrice2e.ContainerOption {
	opts := []gabrice2e.ContainerOption{
		gabrice2e.WithContainerPrivileged(),
		gabrice2e.WithContainerCgroupnsHost(),
		gabrice2e.WithContainerTmpfs("/run"),
		gabrice2e.WithContainerTmpfs("/run/lock"),
		gabrice2e.WithContainerStopSignal("SIGRTMIN+3"),
	}

	if _, err := os.Stat("/sys/fs/cgroup"); err == nil {
		opts = append(opts, gabrice2e.WithContainerVolume("/sys/fs/cgroup", "/sys/fs/cgroup", false))
	}

	return opts
}

func waitForDocker(t *testing.T, g *Target) {
	t.Helper()

	testutils.Eventually(t, 60*time.Second, func() bool {
		return g.Ops().Exec("docker", "info").Capture().RunErr() == nil
	})
	g.Docker = true
}

func waitForSystemd(t *testing.T, g *Target) {
	t.Helper()

	testutils.Eventually(t, 60*time.Second, func() bool {
		e := g.Ops().Exec("systemctl", "show", "-p", "SystemState", "--value").CaptureStdout()
		if err := e.RunErr(); err != nil {
			return false
		}

		state := strings.TrimSpace(e.StdoutStr())
		return state == "running" || state == "degraded"
	})
}
