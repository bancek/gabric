package main

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/sshremote"
)

// S is a short alias for input.Static.
func S[T any](value T) input.Input[T] {
	return input.Static(value)
}

// base is the shared middleware chain for the converge tasks: run with sudo.
var base = gabric.With(gabric.WithRemoteSudo(&gabric.Sudo{}))

// One shared run for every task in an invocation: nodes are evaluated at most
// once and effects flush once. Register it around the whole run and log a
// summary when the run finishes.
var _ = gabric.Use(gabric.WithReconcileReport)

var (
	sshInsecureFlag = gabric.Default.FlagSet.Bool("ssh-insecure", false, "skip host key verification")
	localFlag       = gabric.Default.FlagSet.Bool("local", false, "run against the local machine with LocalRemote")
	tlsFlag         = gabric.Default.FlagSet.Bool("tls", false, "enable ACME/HTTPS")
	domainFlag      = gabric.Default.FlagSet.String("domain", "example.net", "public site domain")
	acmeEmailFlag   = gabric.Default.FlagSet.String("acme-email", "", "ACME account email")
	ageIdentityFlag = gabric.Default.FlagSet.String("age-identity", "", "age identity file")
	ipv6Flag        = gabric.Default.FlagSet.String("ipv6", "", "override IPv6 detection (0 or 1)")
)

var Ssh = gabricssh.RegisterSshTask(gabric.Default)

var Forward = gabricssh.RegisterForwardTask(gabric.Default)

var Hello = gabric.Task("hello", func(ctx context.Context) {
	gabric.TargetOps(ctx).Exec("echo", "hello homelab").Run()
})

func main() {
	gabricssh.InstallDefault()
	gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
		if *sshInsecureFlag {
			config := gabricssh.GetConfig(ctx).
				WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)
			ctx = gabricssh.NewConfigContext(ctx, config)
		}

		cfg := siteConfig{
			Domain: *domainFlag,
			Email:  *acmeEmailFlag,
			TLS:    *tlsFlag,
		}
		if cfg.Email == "" {
			cfg.Email = "admin@" + cfg.Domain
		}
		ctx = withSiteConfig(ctx, cfg)

		if *ageIdentityFlag != "" {
			ctx = withAgeIdentity(ctx, *ageIdentityFlag)
		}

		if *ipv6Flag != "" {
			ctx = withIPv6Override(ctx, *ipv6Flag != "0")
		}

		if *localFlag {
			ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
		}

		return next(ctx)
	}))
}
