package gabrice2e

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricdocker"
)

// DockerTest is a Gabric target backed by a Docker container.
type DockerTest struct {
	*Test
	container *Container
}

// Docker starts a container and returns a Docker target.
func Docker(t testing.TB, image Image, opts ...ContainerOption) *DockerTest {
	t.Helper()

	c := RunContainer(t, image, opts...)

	ctx := t.Context()
	ctx = gabricdocker.NewContainerContext(ctx, c.Name())
	ctx = gabric.NewRemoteRunnerContext(ctx, gabricdocker.Runner())
	ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
	ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)

	return &DockerTest{
		Test: &Test{
			t:   t,
			ctx: ctx,
		},
		container: c,
	}
}

// Container returns the target container.
func (g *DockerTest) Container() *Container {
	return g.container
}
