package main

import (
	"context"

	"github.com/bancek/gabric/reconcile"
)

// siteConfig is the site-wide deployment config, provided by the main
// middleware from the -domain, -acme-email and -tls flags.
type siteConfig struct {
	Domain string
	Email  string
	TLS    bool
}

type siteConfigKey struct{}

func withSiteConfig(ctx context.Context, cfg siteConfig) context.Context {
	return context.WithValue(ctx, siteConfigKey{}, cfg)
}

func siteConfigFrom(ctx context.Context) siteConfig {
	cfg, _ := ctx.Value(siteConfigKey{}).(siteConfig)

	if cfg.Domain == "" {
		cfg.Domain = "example.net"
	}
	if cfg.Email == "" {
		cfg.Email = "admin@" + cfg.Domain
	}

	return cfg
}

// site is a data node: TLS is off by default (plain HTTP on :80), so the
// example converges a host with no public DNS (for example a local VM). Set
// -tls to enable ACME/HTTPS; -domain and -acme-email override the defaults.
var site = reconcile.Data("site", func(ctx context.Context) (siteConfig, error) {
	return siteConfigFrom(ctx), nil
}).Node()
