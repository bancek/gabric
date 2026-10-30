package main

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/ufw"
)

type ipv6OverrideContextKey struct{}

func withIPv6Override(ctx context.Context, value bool) context.Context {
	return context.WithValue(ctx, ipv6OverrideContextKey{}, value)
}

// ipv6 is a data node: host IPv6 availability (override with -ipv6=0/1).
var ipv6 = reconcile.Data("ipv6", func(ctx context.Context) (bool, error) {
	if value, ok := ctx.Value(ipv6OverrideContextKey{}).(bool); ok {
		return value, nil
	}

	e := gabric.TargetOps(ctx).Exec("sh", "-c", "ip -6 route show default 2>/dev/null | grep -q .").Capture()
	err := e.RunErr()
	if err != nil && !modules.IsExitStatus(err, 1) {
		return false, err
	}

	return err == nil, nil
}).Node()

var wireguardRules = []reconcile.NodeRef{
	ufwFirewall.AddRule(ufw.Rule().Port(S("51820")).Proto(S("udp"))),
	ufwFirewall.AddRule(ufw.Rule().In(S("wg0"))),
	ufwFirewall.AddRule(ufw.Rule().Route().In(S("wg0")).Out(S("eth0"))),
	ufwFirewall.AddRule(ufw.Rule().Route().In(S("wg0")).Out(S("wg0"))),
}

// wireguardTemplateData shapes the host's IPv6 support for the compose template.
var wireguardTemplateData = reconcile.Map[bool, any](ipv6,
	func(_ context.Context, v bool) (any, error) {
		return struct{ IPv6 bool }{IPv6: v}, nil
	})

// wireguardComposeFile renders the wg-easy compose from the host's IPv6 support
// (a data node read once per run).
var wireguardComposeFile = modules.File(S("/srv/wireguard/compose.yaml")).
	Content(modules.TemplateData(
		S("/srv/wireguard/compose.yaml"),
		S(wireguardComposeTemplate),
		wireguardTemplateData,
	)).
	Mode(modules.FileMode(0o644)).
	Build().
	Node()

var wireguardCompose = docker.Compose(S("/srv/wireguard/compose.yaml")).
	Build().ApplyNeeds(wireguardComposeFile).Node()

var wireguardNode = reconcile.Group("wireguard").
	Contains(
		modules.Dir(S("/srv/wireguard")).Build().Node(),
	).
	Contains(wireguardRules...).
	Contains(wireguardComposeFile, wireguardCompose).
	Node()

var Wireguard = base.NodeTask(wireguardNode)

// WgUI opens a local port forward to the wg-easy web UI and blocks.
var WgUI = gabric.Task("wg-ui", func(ctx context.Context) {
	gabric.Logger(ctx).Info("open http://127.0.0.1:51821 in your browser")

	gabricssh.Forward(ctx).
		Local("127.0.0.1:51821").
		Remote("127.0.0.1:51821").
		Run()
})
