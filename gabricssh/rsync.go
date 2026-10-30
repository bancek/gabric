package gabricssh

import (
	"context"

	"github.com/kballard/go-shellquote"

	"github.com/bancek/gabric"
)

func sshRsyncRsh(ctx context.Context) (gabric.RsyncRsh, error) {
	hc := GetHostConfig(ctx)

	return gabric.RsyncRsh{
		HostSpec: hc.SSHHost(),
		Rsh:      shellquote.Join(append([]string{"ssh"}, hc.SSHArgs()...)...),
	}, nil
}
