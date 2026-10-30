package modules

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestCommand(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Command(input.List("apt-get", "update")).Node())

	gt.Remote().AssertCommands(gt.Command("apt-get", "update"))
	gt.Remote().AssertCalls()
}
