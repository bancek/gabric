package apt

import (
	"context"
	"strings"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// SatisfiedData reports whether the apt package name satisfies the desired
// installation state, as a data node (dry-run safe, memoized once per run). With
// latest, it also compares the installed version against the apt candidate.
func SatisfiedData(nameInput input.Input[string], latestInput input.Input[bool]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("apt: SatisfiedData: nil name")
	}

	return reconcile.Data(aptSatisfiedName(nameInput, latestInput), func(ctx context.Context) (bool, error) {
		installed, err := InstalledData(nameInput).Value(ctx)
		if err != nil {
			return false, err
		}
		if !installed {
			return false, nil
		}

		latest, err := input.Value(ctx, latestInput)
		if err != nil {
			return false, err
		}
		if !latest {
			return true, nil
		}

		installedVersion, err := VersionData(nameInput).Value(ctx)
		if err != nil {
			return false, err
		}
		candidate, err := CandidateData(nameInput).Value(ctx)
		if err != nil {
			return false, err
		}

		return candidate == "" || candidate == installedVersion, nil
	}).Node()
}

func aptSatisfiedName(nameInput input.Input[string], latestInput input.Input[bool]) string {
	if latestInput == nil {
		return "apt.satisfied:" + nameInput.Name()
	}

	return "apt.satisfied:" + nameInput.Name() + ":" + latestInput.Name()
}

// InstalledData reports whether the apt package name is installed, as a data
// node (dry-run safe, memoized once per run). A dpkg-query "not found" exit is
// false; other errors propagate.
func InstalledData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("apt: InstalledData: nil name")
	}

	return reconcile.Data("apt.installed:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		out, err := modules.CommandOutputData(input.List("dpkg-query", "-W", "-f=${Status}", name)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return false, err
			}

			return false, nil
		}

		return strings.Contains(out, "install ok installed"), nil
	}).Node()
}

// VersionData reads the installed apt package version as a data node
// (dry-run safe, memoized once per run). It returns "" when the package is not
// installed; other errors propagate.
func VersionData(nameInput input.Input[string]) *reconcile.Node[string] {
	if nameInput == nil {
		panic("apt: VersionData: nil name")
	}

	return reconcile.Data("apt.version:"+nameInput.Name(), func(ctx context.Context) (string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("dpkg-query", "-W", "-f=${Version}", name)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return "", err
			}

			return "", nil
		}

		return strings.TrimSpace(out), nil
	}).Node()
}

// CandidateData reads the current apt candidate version as a data node
// (dry-run safe, memoized once per run). It returns "" when the candidate is
// unknown; errors propagate.
func CandidateData(nameInput input.Input[string]) *reconcile.Node[string] {
	if nameInput == nil {
		panic("apt: CandidateData: nil name")
	}

	return reconcile.Data("apt.candidate:"+nameInput.Name(), func(ctx context.Context) (string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("apt-cache", "policy", name)).Value(ctx)
		if err != nil {
			return "", err
		}

		return fieldAfter(out, "Candidate:"), nil
	}).Node()
}
