package linux

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const defaultUserShell = "/bin/bash"

// UserBuilder builds a node that ensures a user account exists.
type UserBuilder struct {
	nameInput       input.Input[string]
	shellInput      input.Input[string]
	homeInput       input.Input[string]
	systemInput     input.Input[bool]
	createHomeInput input.Input[bool]
	groupsInput     input.Input[[]string]
}

// User ensures the user account name exists. By default it is a login user with
// a home directory and /bin/bash as the shell.
func User(nameInput input.Input[string]) *UserBuilder {
	return &UserBuilder{nameInput: nameInput}
}

// Shell sets the login shell.
func (b *UserBuilder) Shell(shellInput input.Input[string]) *UserBuilder {
	b.shellInput = shellInput
	return b
}

// Home sets the home directory. An empty home uses useradd's default.
func (b *UserBuilder) Home(homeInput input.Input[string]) *UserBuilder {
	b.homeInput = homeInput
	return b
}

// System creates a system account. It does not create a home directory unless
// CreateHome is called.
func (b *UserBuilder) System(flag ...input.Input[bool]) *UserBuilder {
	b.systemInput = input.Flag(flag...)
	return b
}

// CreateHome creates the home directory.
func (b *UserBuilder) CreateHome(flag ...input.Input[bool]) *UserBuilder {
	b.createHomeInput = input.Flag(flag...)
	return b
}

// Groups ensures the user is a member of the given supplementary groups.
// Membership is additive: groups not listed are left unchanged.
func (b *UserBuilder) Groups(groupsInput input.Input[[]string]) *UserBuilder {
	b.groupsInput = groupsInput
	return b
}

// Build hands off to the reconcile node builder.
func (b *UserBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("linux: User: nil name")
	}

	nameInput := b.nameInput
	resourceName := "linux.user:" + nameInput.Name()

	return reconcile.ResourceBool(resourceName,
		func(ctx context.Context) (bool, error) {
			shellInput := b.shellInput
			if shellInput == nil {
				shellInput = input.Static(defaultUserShell)
			}

			return UserSatisfiedData(
				nameInput,
				shellInput,
				b.homeInput,
				b.groupsInput,
			).Value(ctx)
		},
		func(ctx context.Context) error {
			name, err := nameInput.Value(ctx)
			if err != nil {
				return err
			}

			shell, err := input.ValueOr(ctx, b.shellInput, defaultUserShell)
			if err != nil {
				return err
			}

			home, err := input.Value(ctx, b.homeInput)
			if err != nil {
				return err
			}

			system, err := input.Value(ctx, b.systemInput)
			if err != nil {
				return err
			}

			createHome := !system
			if b.createHomeInput != nil {
				createHome, err = b.createHomeInput.Value(ctx)
				if err != nil {
					return err
				}
			}

			groups, err := input.Value(ctx, b.groupsInput)
			if err != nil {
				return err
			}

			passwd, err := UserPasswdData(nameInput).Value(ctx)
			if err != nil {
				return err
			}

			if !passwd.Exists {
				args := []string{"useradd", "--user-group"}

				if system {
					args = append(args, "--system")
				}

				if createHome {
					args = append(args, "--create-home")
				} else {
					args = append(args, "--no-create-home")
				}

				if home != "" {
					args = append(args, "--home-dir", home)
				}

				if shell != "" {
					args = append(args, "--shell", shell)
				}

				if err := gabric.TargetOps(ctx).Exec(append(args, name)...).RunErr(); err != nil {
					return err
				}
			} else {
				if shell != "" && passwd.Shell != shell {
					if err := gabric.TargetOps(ctx).Exec("usermod", "--shell", shell, name).RunErr(); err != nil {
						return err
					}
				}

				if home != "" && passwd.Home != home {
					if err := gabric.TargetOps(ctx).Exec("usermod", "--home", home, name).RunErr(); err != nil {
						return err
					}
				}
			}

			if len(groups) > 0 {
				if err := gabric.TargetOps(ctx).Exec("usermod", "-aG", strings.Join(groups, ","), name).RunErr(); err != nil {
					return err
				}
			}

			return nil
		})
}
