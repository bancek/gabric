package e2eimages

import (
	_ "embed"

	"github.com/bancek/gabric/gabrice2e"
)

//go:embed Dockerfile.alpine-rsync
var alpineRsync []byte

//go:embed Dockerfile.alpine-openssh
var alpineOpenSSH []byte

var (
	// DockerRsync is a minimal Alpine target with rsync and sudo.
	DockerRsync = gabrice2e.ImageFromDockerfileBytes("docker-rsync", alpineRsync)

	// DockerSSH is an Alpine target running OpenSSH with the gabric and app
	// users, sudo, and rsync.
	DockerSSH = gabrice2e.ImageFromDockerfileBytes("docker-ssh", alpineOpenSSH)
)
