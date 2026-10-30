package moduletest

import (
	_ "embed"

	"github.com/bancek/gabric/gabrice2e"
)

var (
	//go:embed testdata/Dockerfile.alpine
	alpineDockerfile []byte
	//go:embed testdata/Dockerfile.alpine-user
	alpineUserDockerfile []byte
	//go:embed testdata/Dockerfile.ubuntu
	ubuntuDockerfile []byte
	//go:embed testdata/Dockerfile.ubuntu-user
	ubuntuUserDockerfile []byte
	//go:embed testdata/Dockerfile.alpine-tools
	alpineToolsDockerfile []byte
	//go:embed testdata/Dockerfile.ubuntu-tools
	ubuntuToolsDockerfile []byte
	//go:embed testdata/Dockerfile.http
	httpDockerfile []byte
	//go:embed testdata/Dockerfile.ubuntu-systemd
	ubuntuSystemdDockerfile []byte
	//go:embed testdata/Dockerfile.ubuntu-ufwdocker
	ubuntuUfwDockerDockerfile []byte
	//go:embed testdata/Dockerfile.dind
	dindDockerfile []byte
)

var (
	// Alpine is a minimal Alpine target.
	Alpine = gabrice2e.ImageFromDockerfileBytes("alpine", alpineDockerfile)
	// AlpineUser is a minimal Alpine target running as the app user.
	AlpineUser = gabrice2e.ImageFromDockerfileBytes("alpine-user", alpineUserDockerfile)
	// Ubuntu is a minimal Ubuntu target.
	Ubuntu = gabrice2e.ImageFromDockerfileBytes("ubuntu", ubuntuDockerfile)
	// UbuntuUser is a minimal Ubuntu target running as the app user.
	UbuntuUser = gabrice2e.ImageFromDockerfileBytes("ubuntu-user", ubuntuUserDockerfile)
	// AlpineTools has the Linux tooling used by module e2e tests.
	AlpineTools = gabrice2e.ImageFromDockerfileBytes("alpine-tools", alpineToolsDockerfile)
	// UbuntuTools has the Linux tooling used by module e2e tests.
	UbuntuTools = gabrice2e.ImageFromDockerfileBytes("ubuntu-tools", ubuntuToolsDockerfile)
	// HTTP serves a file tree with BusyBox httpd for HTTP e2e tests.
	HTTP = gabrice2e.ImageFromDockerfileBytes("http", httpDockerfile)
	// UbuntuSystemd is an Ubuntu target running systemd.
	UbuntuSystemd = gabrice2e.ImageFromDockerfileBytes("ubuntu-systemd", ubuntuSystemdDockerfile)
	// UbuntuUfwDocker is an Ubuntu systemd target with ufw and Docker.
	UbuntuUfwDocker = gabrice2e.ImageFromDockerfileBytes("ubuntu-ufwdocker", ubuntuUfwDockerDockerfile)
	// Dind is a Docker-in-Docker target.
	Dind = gabrice2e.ImageFromDockerfileBytes("dind", dindDockerfile)
)
