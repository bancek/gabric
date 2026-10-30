package linux

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
)

func statCall(path string) gabrictest.Call {
	return gabrictest.Call{Stat: &gabric.StatRequest{Path: path}}
}

func sudoCommand(gt *gabrictest.Test, name string, args ...string) gabrictest.Command {
	cmd := gt.Command(name, args...)
	cmd.Sudo = &gabrictest.Sudo{}

	return cmd
}
