package gabricssh

import (
	"testing"

	"github.com/bancek/gabric"
)

func TestRegisterSshTask(t *testing.T) {
	g := gabric.New()

	RegisterSshTask(g)

	if _, err := g.ParseTasks([]string{"ssh"}); err != nil {
		t.Fatalf("ParseTasks(ssh) error: %v", err)
	}
}

func TestRegisterSshTaskDuplicate(t *testing.T) {
	g := gabric.New()

	RegisterSshTask(g)

	defer func() {
		if recover() == nil {
			t.Fatal("RegisterSshTask() twice did not panic")
		}
	}()

	RegisterSshTask(g)
}
