package gabricssh

import (
	"context"
	"slices"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
)

type sshInstallFlags struct {
	host       string
	identities []string
}

func (f *sshInstallFlags) addIdentity(path string) {
	f.identities = append(f.identities, path)
}

func (f *sshInstallFlags) identityPaths() []string {
	return slices.Clone(f.identities)
}

// Install registers the SSH flags and setup middleware on g. It must run before
// Main parses flags.
func Install(g *gabric.Gabric) {
	if g.FlagSet.Lookup("ssh-host") != nil {
		panic("gabricssh: already installed")
	}

	flags := &sshInstallFlags{}

	g.FlagSet.StringVar(&flags.host, "ssh-host", "", "SSH host (user@host:port)")
	g.FlagSet.Func("ssh-identity", "SSH identity file (repeatable)", func(value string) error {
		flags.addIdentity(value)
		return nil
	})

	g.UseBuiltin(
		WithSSH,
		withSSHHost(flags),
		withSSHIdentity(flags),
	)
}

// InstallDefault installs SSH on gabric.Default.
func InstallDefault() {
	Install(gabric.Default)
}

func withSSHHost(flags *sshInstallFlags) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		if flags.host == "" || HasHost(ctx) {
			return next(ctx)
		}

		return WithHost(flags.host)(ctx, next)
	}
}

func withSSHIdentity(flags *sshInstallFlags) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		identities := flags.identityPaths()
		if len(identities) == 0 {
			return next(ctx)
		}

		config := GetConfig(ctx)
		ctx = NewConfigContext(ctx, config.WithPrivateKeysPaths(
			append(config.PrivateKeysPaths(), identities...),
		))

		return next(ctx)
	}
}
