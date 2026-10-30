package docker

import (
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
)

const composePath = "/srv/www/compose.yaml"

const (
	composeRunning = `{"id":"Container www","status":"Done","text":"Running"}` + "\n"
	composeDrift   = `{"id":"Container www","status":"Done","text":"Created"}` + "\n"
)

func composeProbeCommand(gt *gabrictest.Test) gabrictest.Command {
	return gt.Command("docker", "compose", "-f", composePath,
		"--progress", "json", "--dry-run", "up", "-d", "--remove-orphans")
}

func composeApplyCommand(gt *gabrictest.Test, build bool) gabrictest.Command {
	args := []string{"compose", "-f", composePath, "up", "-d"}
	if build {
		args = append(args, "--build")
	}
	args = append(args, "--remove-orphans")

	return gt.Command("docker", args...)
}

// registerDockerHandler answers the dry-run probe on stderr (drift or satisfied)
// and exits 0 otherwise.
func registerDockerHandler(side gabrictest.Side, drift bool) {
	side.Handle("docker", func(cmd gabrictest.Command) gabrictest.Result {
		if !cmd.HasArg("--dry-run") {
			return gabrictest.Result{}
		}

		if drift {
			return gabrictest.Result{Stderr: composeDrift}
		}

		return gabrictest.Result{Stderr: composeRunning}
	})
}

func TestComposeSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), false)

	if !gt.RunNode(gt.Context(), ComposeSatisfiedData(input.Static(composePath))) {
		t.Error("ComposeSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(composeProbeCommand(gt))
}

func TestComposeUp(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), true)

	gt.RunNode(gt.Context(), Compose(input.Static(composePath)).Build().Node())

	gt.Remote().AssertCommands(composeProbeCommand(gt), composeApplyCommand(gt, false))
}

func TestComposeUpToDate(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), false)

	gt.RunNode(gt.Context(), Compose(input.Static(composePath)).Build().Node())

	gt.Remote().AssertCommands(composeProbeCommand(gt))
}

func TestComposeBuildImages(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), true)

	gt.RunNode(gt.Context(), Compose(input.Static(composePath)).BuildImages().Build().Node())

	gt.Remote().AssertCommands(composeProbeCommand(gt), composeApplyCommand(gt, true))
}

func TestComposeDryRun(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), true)

	ctx := dryrun.With(gt.Context())

	gt.RunNode(ctx, Compose(input.Static(composePath)).Build().Node())

	gt.Remote().AssertCommands(composeProbeCommand(gt))
}

func TestComposeMissingFile(t *testing.T) {
	gt := gabrictest.New(t)
	registerDockerHandler(gt.Remote(), true)

	gt.RunNode(gt.Context(), Compose(input.Static(composePath)).Build().Node())

	gt.Remote().AssertCommands(composeApplyCommand(gt, false))
}

func TestComposeNeeds(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(composePath, []byte("services: {}\n"), 0o644)
	registerDockerHandler(gt.Remote(), true)

	gt.Remote().Handle("touch", func(cmd gabrictest.Command) gabrictest.Result {
		for _, path := range cmd.Args {
			gt.Remote().WriteFile(path, nil, 0o644)
		}

		return gabrictest.Result{}
	})

	dep := modules.Command(input.List("touch", composePath)).Node()
	gt.RunNode(gt.Context(), Compose(input.Static(composePath)).Build().ApplyNeeds(dep).Node())

	gt.Remote().AssertCommands(
		composeProbeCommand(gt),
		gt.Command("touch", composePath),
		composeApplyCommand(gt, false),
	)
}

func TestComposeBuildPanics(t *testing.T) {
	assertPanics(t, "nil file", func() { Compose(nil).Build() })
}

func TestComposeWouldChange(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"running", composeRunning, false},
		{"running status", `{"dry-run":true,"id":"Container www","status":"Running"}`, false},
		{"created", composeDrift, true},
		{"created status", `{"dry-run":true,"id":"Container www","status":"Created"}`, true},
		{"recreate", `{"id":"Container www","status":"Working","text":"Recreate"}`, true},
		{"empty", "", false},
		{"warning ignored", "WARN[0000] no config file", false},
		{"mixed", "WARN[0000] x\n" + composeRunning + `{"id":"Container x","status":"Done","text":"Started"}`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeWouldChange(tc.out); got != tc.want {
				t.Errorf("composeWouldChange() = %v, want %v", got, tc.want)
			}
		})
	}
}
