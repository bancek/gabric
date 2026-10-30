package gabrice2e

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/bancek/gabric"
)

// Container is a live Docker container started by RunContainer.
type Container struct {
	t     testing.TB
	name  string
	image Image
	ports []int

	stopOnce sync.Once
}

// RunContainer builds image when needed and starts a new container. It always
// runs with --rm, so Docker removes the container when it stops.
func RunContainer(t testing.TB, image Image, opts ...ContainerOption) *Container {
	t.Helper()
	requireDocker(t)

	cfg := applyContainerOptions(opts)
	if cfg.name == "" {
		cfg.name = "gabrice2e-" + uuid.NewV4().String()
	}

	ref := image.build(t)
	args := cfg.runArgs(ref)

	if _, err := runDocker(t.Context(), args...); err != nil {
		t.Fatalf("gabrice2e: docker %s: %v", strings.Join(args, " "), err)
	}

	c := &Container{
		t:     t,
		name:  cfg.name,
		image: image,
		ports: append([]int(nil), cfg.ports...),
	}

	c.requireRunning()

	t.Cleanup(c.Stop)

	return c
}

// Name returns the container name.
func (c *Container) Name() string {
	return c.name
}

// Image returns the image used for the container.
func (c *Container) Image() Image {
	return c.image
}

// Port returns the host port published for containerPort.
func (c *Container) Port(containerPort int) int {
	c.t.Helper()

	if !slices.Contains(c.ports, containerPort) {
		c.t.Fatalf("gabrice2e: container port %d was not published", containerPort)
	}

	out := c.docker("port", c.name, fmt.Sprintf("%d/tcp", containerPort))

	_, portStr, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		c.t.Fatalf("gabrice2e: docker port %s %d: %v (output %q)", c.name, containerPort, err, out)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.t.Fatalf("gabrice2e: docker port %s %d: %v (output %q)", c.name, containerPort, err, out)
	}

	return port
}

// Addr returns 127.0.0.1:<host-port> for containerPort.
func (c *Container) Addr(containerPort int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(c.Port(containerPort)))
}

// Logs returns the container's logs. It is best-effort and returns a diagnostic
// string when docker logs fails.
func (c *Container) Logs() string {
	out, err := runDocker(context.Background(), "logs", c.name)
	if err != nil {
		return fmt.Sprintf("gabrice2e: docker logs %s: %v", c.name, err)
	}

	return out
}

// Stop stops the container. It is best-effort and safe to call more than once.
func (c *Container) Stop() {
	c.stopOnce.Do(func() {
		if c.t.Failed() {
			c.t.Logf("gabrice2e: container %s logs:\n%s", c.name, c.Logs())
		}

		if _, err := runDocker(context.Background(), "stop", "-t", "1", c.name); err != nil {
			_, _ = runDocker(context.Background(), "rm", "-f", c.name)
		}
	})
}

func (c *Container) requireRunning() {
	c.t.Helper()

	out, err := c.dockerErr("inspect", "-f", "{{.State.Running}}", c.name)
	if err != nil {
		c.t.Fatalf("gabrice2e: container %s exited and may have been removed by --rm: %v\nlogs:\n%s", c.name, err, c.Logs())
	}

	if strings.TrimSpace(out) != "true" {
		c.t.Fatalf("gabrice2e: container %s is not running\nlogs:\n%s", c.name, c.Logs())
	}
}

func (c *Container) docker(args ...string) string {
	c.t.Helper()

	out, err := c.dockerErr(args...)
	if err != nil {
		c.t.Fatalf("gabrice2e: docker %s: %v", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(out)
}

func (c *Container) dockerErr(args ...string) (string, error) {
	return runDocker(c.t.Context(), args...)
}

func runDocker(ctx context.Context, args ...string) (string, error) {
	e := gabric.Local(ctx).Exec(append([]string{"docker"}, args...)...).Capture()
	if err := e.RunErr(); err != nil {
		return "", fmt.Errorf("%w\nstdout:\n%s\nstderr:\n%s", err, e.StdoutStr(), e.StderrStr())
	}

	return e.StdoutStr(), nil
}

var (
	dockerOnce sync.Once
	dockerOK   bool
)

func requireDocker(t testing.TB) {
	t.Helper()

	if os.Getenv("GABRIC_E2E_DOCKER_DISABLED") != "" {
		t.Skip("gabrice2e: docker disabled by GABRIC_E2E_DOCKER_DISABLED")
	}

	dockerOnce.Do(func() {
		if _, err := exec.LookPath("docker"); err != nil {
			return
		}

		_, err := runDocker(context.Background(), "info")
		dockerOK = err == nil
	})

	if !dockerOK {
		t.Skip("gabrice2e: docker unavailable")
	}
}
