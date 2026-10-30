package apk

import (
	"context"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// SatisfiedData reports whether the apk package name satisfies the desired
// installation state, as a data node (dry-run safe, memoized once per run). With
// latest, it also compares the installed version against the apk candidate.
func SatisfiedData(nameInput input.Input[string], latestInput input.Input[bool]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("apk: SatisfiedData: nil name")
	}

	return reconcile.Data(apkSatisfiedName(nameInput, latestInput), func(ctx context.Context) (bool, error) {
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

func apkSatisfiedName(nameInput input.Input[string], latestInput input.Input[bool]) string {
	if latestInput == nil {
		return "apk.satisfied:" + nameInput.Name()
	}

	return "apk.satisfied:" + nameInput.Name() + ":" + latestInput.Name()
}

// InstalledData reports whether the apk package name is installed, as a data
// node (dry-run safe, memoized once per run). An apk info "not found" exit is
// false; other errors propagate.
func InstalledData(nameInput input.Input[string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("apk: InstalledData: nil name")
	}

	return reconcile.Data("apk.installed:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return false, err
		}

		_, err = modules.CommandOutputData(input.List("apk", "info", "-e", name)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return false, err
			}

			return false, nil
		}

		return true, nil
	}).Node()
}

// VersionData reads the installed apk package version as a data node (dry-run
// safe, memoized once per run). It returns "" when the package is not installed;
// other errors propagate.
func VersionData(nameInput input.Input[string]) *reconcile.Node[string] {
	if nameInput == nil {
		panic("apk: VersionData: nil name")
	}

	return reconcile.Data("apk.version:"+nameInput.Name(), func(ctx context.Context) (string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("apk", "list", "-I", name)).Value(ctx)
		if err != nil {
			return "", err
		}

		return versionFrom(out, name), nil
	}).Node()
}

// CandidateData reads the current apk candidate version as a data node (dry-run
// safe, memoized once per run). It returns "" when the candidate is unknown;
// errors propagate.
func CandidateData(nameInput input.Input[string]) *reconcile.Node[string] {
	if nameInput == nil {
		panic("apk: CandidateData: nil name")
	}

	return reconcile.Data("apk.candidate:"+nameInput.Name(), func(ctx context.Context) (string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("apk", "search", "-x", name)).Value(ctx)
		if err != nil {
			return "", err
		}

		return versionFrom(out, name), nil
	}).Node()
}
