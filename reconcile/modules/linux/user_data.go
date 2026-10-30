package linux

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// UserPasswd is one entry from getent passwd.
type UserPasswd struct {
	Name     string
	Password string
	UID      string
	GID      string
	GECOS    string
	Home     string
	Shell    string
	Exists   bool
}

// UserPasswdData reads name's passwd entry as a data node (dry-run safe,
// memoized once per run). A getent "not found" exit yields Exists=false; other
// errors propagate.
func UserPasswdData(nameInput input.Input[string]) *reconcile.Node[UserPasswd] {
	if nameInput == nil {
		panic("linux: UserPasswdData: nil name")
	}

	return reconcile.Data("linux.user-passwd:"+nameInput.Name(), func(ctx context.Context) (UserPasswd, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return UserPasswd{}, err
		}

		out, err := modules.CommandOutputData(input.List("getent", "passwd", name)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 2) {
				return UserPasswd{}, err
			}

			return UserPasswd{}, nil
		}

		fields := strings.SplitN(strings.TrimSpace(out), ":", 7)
		if len(fields) != 7 {
			return UserPasswd{}, fmt.Errorf("getent passwd %s: unexpected output %q", name, out)
		}

		return UserPasswd{
			Name:     fields[0],
			Password: fields[1],
			UID:      fields[2],
			GID:      fields[3],
			GECOS:    fields[4],
			Home:     fields[5],
			Shell:    fields[6],
			Exists:   true,
		}, nil
	}).Node()
}

// UserSatisfiedData reports whether name matches the desired shell, home and
// supplementary groups, as a data node (dry-run safe, memoized once per run).
// Empty shell or home is not checked; groups are additive membership checks.
func UserSatisfiedData(nameInput, shellInput, homeInput input.Input[string], groupsInput input.Input[[]string]) *reconcile.Node[bool] {
	if nameInput == nil {
		panic("linux: UserSatisfiedData: nil name")
	}

	return reconcile.Data("linux.user-satisfied:"+nameInput.Name(), func(ctx context.Context) (bool, error) {
		passwd, err := UserPasswdData(nameInput).Value(ctx)
		if err != nil {
			return false, err
		}
		if !passwd.Exists {
			return false, nil
		}

		shell, err := input.Value(ctx, shellInput)
		if err != nil {
			return false, err
		}
		if shell != "" && passwd.Shell != shell {
			return false, nil
		}

		home, err := input.Value(ctx, homeInput)
		if err != nil {
			return false, err
		}
		if home != "" && passwd.Home != home {
			return false, nil
		}

		groups, err := input.Value(ctx, groupsInput)
		if err != nil {
			return false, err
		}
		if len(groups) > 0 {
			have, err := UserGroupsData(nameInput).Value(ctx)
			if err != nil {
				return false, err
			}

			for _, group := range groups {
				if !slices.Contains(have, group) {
					return false, nil
				}
			}
		}

		return true, nil
	}).Node()
}

// UserGroupData reads name's primary group name as a data node (dry-run safe,
// memoized once per run).
func UserGroupData(nameInput input.Input[string]) *reconcile.Node[string] {
	if nameInput == nil {
		panic("linux: UserGroupData: nil name")
	}

	return reconcile.Data("linux.user-group:"+nameInput.Name(), func(ctx context.Context) (string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return "", err
		}

		out, err := modules.CommandOutputData(input.List("id", "-gn", name)).Value(ctx)
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(out), nil
	}).Node()
}

// UserGroupsData reads name's group names (primary and supplementary) as a data
// node (dry-run safe, memoized once per run).
func UserGroupsData(nameInput input.Input[string]) *reconcile.Node[[]string] {
	if nameInput == nil {
		panic("linux: UserGroupsData: nil name")
	}

	return reconcile.Data("linux.user-groups:"+nameInput.Name(), func(ctx context.Context) ([]string, error) {
		name, err := nameInput.Value(ctx)
		if err != nil {
			return nil, err
		}

		out, err := modules.CommandOutputData(input.List("id", "-nG", name)).Value(ctx)
		if err != nil {
			return nil, err
		}

		return strings.Fields(out), nil
	}).Node()
}
