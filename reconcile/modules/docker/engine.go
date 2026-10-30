package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/apt"
	"github.com/bancek/gabric/reconcile/modules/httpfile"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

const (
	dockerKeyringPath = "/etc/apt/keyrings/docker.asc"
	dockerSourcePath  = "/etc/apt/sources.list.d/docker.list"
)

var dockerPackages = []string{
	"docker-ce",
	"docker-ce-cli",
	"containerd.io",
	"docker-buildx-plugin",
	"docker-compose-plugin",
}

type engineRepo struct {
	Distro   string
	Codename string
	Arch     string
}

func dockerDistro(release linux.OSRelease) (string, error) {
	switch {
	case release.ID == "ubuntu" || release.IsLike("ubuntu"):
		return "ubuntu", nil
	case release.ID == "debian" || release.IsLike("debian"):
		return "debian", nil
	default:
		return "", fmt.Errorf("docker: unsupported distribution %q", release.ID)
	}
}

func dockerKeyURL(distro string) string {
	return "https://download.docker.com/linux/" + distro + "/gpg"
}

func dockerSourceLine(repo engineRepo) string {
	return fmt.Sprintf(
		"deb [arch=%s signed-by=%s] https://download.docker.com/linux/%s %s stable\n",
		repo.Arch, dockerKeyringPath, repo.Distro, repo.Codename,
	)
}

func engineRepoData() *reconcile.Node[engineRepo] {
	return reconcile.Data("docker.engine-repo", func(ctx context.Context) (engineRepo, error) {
		release, err := linux.OSReleaseData().Value(ctx)
		if err != nil {
			return engineRepo{}, err
		}

		distro, err := dockerDistro(release)
		if err != nil {
			return engineRepo{}, err
		}
		if release.VersionCodename == "" {
			return engineRepo{}, fmt.Errorf("docker: VERSION_CODENAME missing for %s", release.ID)
		}

		out, err := modules.CommandOutputData(input.List("dpkg", "--print-architecture")).Value(ctx)
		if err != nil {
			return engineRepo{}, err
		}

		arch := strings.TrimSpace(out)
		if arch == "" {
			return engineRepo{}, fmt.Errorf("docker: dpkg --print-architecture returned no architecture")
		}

		return engineRepo{Distro: distro, Codename: release.VersionCodename, Arch: arch}, nil
	}).Node()
}

// EngineGroup is the Docker Engine installation: apt repository, packages and
// service.
type EngineGroup struct {
	*reconcile.Node[reconcile.Void]

	Keyring  *reconcile.Node[bool]
	Source   *reconcile.Node[bool]
	Update   *reconcile.Node[bool]
	Packages []*reconcile.Node[bool]
	Service  *reconcile.Node[bool]
}

// EngineBuilder configures a Docker Engine installation.
type EngineBuilder struct{}

// DockerEngine starts a Docker Engine builder.
func DockerEngine() *EngineBuilder {
	return &EngineBuilder{}
}

// Build builds the Docker Engine installation group.
func (b *EngineBuilder) Build() *EngineGroup {
	repoData := engineRepoData()

	keyring := reconcile.DynamicResource("docker.engine-keyring",
		func(ctx context.Context) (*reconcile.ResourceBuilder[bool], error) {
			repo, err := repoData.Value(ctx)
			if err != nil {
				return nil, err
			}

			return httpfile.File(input.Static(dockerKeyURL(repo.Distro))).
				Path(input.Static(dockerKeyringPath)).
				Mode(modules.FileMode(0o644)).
				Build(), nil
		}).Node()

	source := modules.File(input.Static(dockerSourcePath)).
		Content(reconcile.Map(repoData, func(ctx context.Context, repo engineRepo) ([]byte, error) {
			return []byte(dockerSourceLine(repo)), nil
		})).
		Mode(modules.FileMode(0o644)).
		Build().
		Node()

	update := apt.Update().Build().
		ApplyNeeds(keyring, source).
		Node()

	packages := make([]*reconcile.Node[bool], 0, len(dockerPackages))
	packageRefs := make([]reconcile.NodeRef, 0, len(dockerPackages))
	for _, name := range dockerPackages {
		node := apt.Package(input.Static(name)).Build().ApplyNeeds(update).Node()
		packages = append(packages, node)
		packageRefs = append(packageRefs, node)
	}

	packageGroup := reconcile.Group("docker.engine-packages").
		Contains(packageRefs...).
		Node()

	service := systemd.Service(input.Static("docker")).Started().Build().
		CheckNeeds(packageGroup).
		Node()

	node := reconcile.Group("docker.engine").
		Contains(keyring, source).
		Contains(packageRefs...).
		Contains(service).
		Node()

	return &EngineGroup{
		Node:     node,
		Keyring:  keyring,
		Source:   source,
		Update:   update,
		Packages: packages,
		Service:  service,
	}
}
