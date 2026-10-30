package ufw

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// StatusInfo is the parsed output of `ufw status verbose`.
type StatusInfo struct {
	Active   bool
	Incoming string
	Outgoing string
	Routed   string
}

// StatusData reads `ufw status verbose` as a data node (dry-run safe,
// memoized once per run). The firewall cover creates two of these: an internal
// probe the default-policy and enabled nodes read, and an observed one they
// invalidate when they change (so a later read re-probes).
func StatusData() *reconcile.Node[StatusInfo] {
	return reconcile.Data("ufw.status", func(ctx context.Context) (StatusInfo, error) {
		out, err := modules.CommandOutputData(input.List("ufw", "status", "verbose")).Value(ctx)
		if err != nil {
			return StatusInfo{}, err
		}

		return parseUFWStatus(out), nil
	}).Node()
}

// DefaultIncoming ensures the default incoming policy (ufw default <policy>
// incoming). It panics if policy is nil. The node yields bool (changed) and
// does not notify by itself.
func DefaultIncoming(status *reconcile.Node[StatusInfo], policyInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	return ufwDefault(status, policyInput, "incoming", func(info StatusInfo) string {
		return info.Incoming
	})
}

// DefaultOutgoing ensures the default outgoing policy (ufw default <policy>
// outgoing). It panics if policy is nil. The node yields bool (changed) and
// does not notify by itself.
func DefaultOutgoing(status *reconcile.Node[StatusInfo], policyInput input.Input[string]) *reconcile.ResourceBuilder[bool] {
	return ufwDefault(status, policyInput, "outgoing", func(info StatusInfo) string {
		return info.Outgoing
	})
}

func ufwDefault(status *reconcile.Node[StatusInfo], policyInput input.Input[string], direction string, current func(StatusInfo) string) *reconcile.ResourceBuilder[bool] {
	if policyInput == nil {
		panic("modules: UFW: nil default " + direction + " policy")
	}

	return reconcile.ResourceBool("ufw.default-"+direction,
		func(ctx context.Context) (bool, error) {
			policy, err := policyInput.Value(ctx)
			if err != nil {
				return false, err
			}

			return reconcile.ValueMatches(status, func(info StatusInfo) bool {
				return current(info) == policy
			})(ctx)
		},
		func(ctx context.Context) error {
			policy, err := policyInput.Value(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec("ufw", "default", policy, direction).RunErr()
		})
}

// Enabled ensures the firewall is enabled (or disabled). The node yields
// bool (changed) and does not notify by itself.
func Enabled(status *reconcile.Node[StatusInfo], enabledInput input.Input[bool]) *reconcile.ResourceBuilder[bool] {
	if enabledInput == nil {
		panic("modules: UFW: nil enabled input")
	}

	return reconcile.ResourceBool("ufw.enabled",
		func(ctx context.Context) (bool, error) {
			enabled, err := enabledInput.Value(ctx)
			if err != nil {
				return false, err
			}

			return reconcile.ValueMatches(status, func(info StatusInfo) bool {
				return info.Active == enabled
			})(ctx)
		},
		func(ctx context.Context) error {
			enabled, err := enabledInput.Value(ctx)
			if err != nil {
				return err
			}

			if enabled {
				return gabric.TargetOps(ctx).Exec("ufw", "--force", "enable").RunErr()
			}

			return gabric.TargetOps(ctx).Exec("ufw", "--force", "disable").RunErr()
		})
}

// FirewallBuilder configures the firewall cover. Enabled is required and rules
// registered with Rule are evaluated before the default policies and the
// enabled state.
type FirewallBuilder struct {
	incomingInput input.Input[string]
	outgoingInput input.Input[string]
	enabledInput  input.Input[bool]
	rules         []*RuleBuilder
}

// Firewall starts a firewall cover builder.
func Firewall() *FirewallBuilder {
	return &FirewallBuilder{}
}

// Incoming sets the default incoming policy ("deny", "allow", ...).
func (b *FirewallBuilder) Incoming(policyInput input.Input[string]) *FirewallBuilder {
	b.incomingInput = policyInput
	return b
}

// Outgoing sets the default outgoing policy.
func (b *FirewallBuilder) Outgoing(policyInput input.Input[string]) *FirewallBuilder {
	b.outgoingInput = policyInput
	return b
}

// Enabled enables or disables the firewall (ufw --force enable/disable).
func (b *FirewallBuilder) Enabled(flag ...input.Input[bool]) *FirewallBuilder {
	b.enabledInput = input.Flag(flag...)
	return b
}

// Rule adds a rule to the cover. Registered rules are evaluated before the
// default policies and the enabled state, so a port is allowed before the
// firewall is enabled with a default-deny policy.
func (b *FirewallBuilder) Rule(rule *RuleBuilder) *FirewallBuilder {
	b.rules = append(b.rules, rule)
	return b
}

// Build snapshots the cover. It panics unless Enabled was called.
func (b *FirewallBuilder) Build() *FirewallGroup {
	if b.enabledInput == nil {
		panic("modules: UFW: call Enabled")
	}

	// probe is the shared status the default/enabled nodes read; status is the
	// observed one they invalidate, so a later read re-probes. They must be
	// distinct nodes: the engine rejects invalidating a node the mutator reads.
	probe := StatusData()
	status := probe.Clone()

	rules := make([]*reconcile.Node[bool], 0, len(b.rules))
	ruleRefs := make([]reconcile.NodeRef, 0, len(b.rules))
	for _, rule := range b.rules {
		node := rule.Build().ApplyInvalidates(status).Node()
		rules = append(rules, node)
		ruleRefs = append(ruleRefs, node)
	}

	enabled := Enabled(probe, b.enabledInput).ApplyInvalidates(status).Node()

	var incoming, outgoing *reconcile.Node[bool]

	if b.incomingInput != nil {
		incoming = DefaultIncoming(probe, b.incomingInput).ApplyInvalidates(status).Node()
	}

	if b.outgoingInput != nil {
		outgoing = DefaultOutgoing(probe, b.outgoingInput).ApplyInvalidates(status).Node()
	}

	// Order matters: rules first (allow before default-deny and enable), then
	// the probe, then the defaults and enabled. The observed status is read
	// last, after the mutators invalidated it: the node must not have read it
	// before they run.
	node := reconcile.Group("ufw.firewall").Contains(ruleRefs...).Contains(probe)

	if incoming != nil {
		node = node.Contains(incoming)
	}

	if outgoing != nil {
		node = node.Contains(outgoing)
	}

	node = node.Contains(enabled).Contains(status)

	return &FirewallGroup{
		Node:            node.Node(),
		Status:          status,
		Probe:           probe,
		Rules:           rules,
		DefaultIncoming: incoming,
		DefaultOutgoing: outgoing,
		Enabled:         enabled,
	}
}

// FirewallGroup is the firewall cover: a group of the rules, the probe, the default
// policies, the enabled state and the observed status. Reading it runs them in
// that order. It is not memoized and owns nothing; start a new Run to reset it.
type FirewallGroup struct {
	*reconcile.Node[reconcile.Void]

	Status          *reconcile.Node[StatusInfo]
	Probe           *reconcile.Node[StatusInfo]
	Rules           []*reconcile.Node[bool]
	DefaultIncoming *reconcile.Node[bool]
	DefaultOutgoing *reconcile.Node[bool]
	Enabled         *reconcile.Node[bool]
}

// AddRule builds a UFW rule that invalidates the observed status when it
// applies, so a later read re-probes. It does not add the rule to the cover;
// group the returned node into the service that owns the rule.
func (u *FirewallGroup) AddRule(rule *RuleBuilder) *reconcile.Node[bool] {
	return rule.Build().ApplyInvalidates(u.Status).Node()
}

// parseUFWStatus parses `ufw status verbose`. When the firewall is inactive the
// output has no Default line, so the policies are left empty (which reads as
// drift and gets applied).
func parseUFWStatus(out string) StatusInfo {
	info := StatusInfo{Active: fieldAfter(out, "Status:") == "active"}

	for part := range strings.SplitSeq(fieldAfter(out, "Default:"), ",") {
		policy, dir, ok := strings.Cut(strings.TrimSpace(part), " (")
		if !ok {
			continue
		}

		switch strings.TrimSuffix(dir, ")") {
		case "incoming":
			info.Incoming = policy
		case "outgoing":
			info.Outgoing = policy
		case "routed":
			info.Routed = policy
		}
	}

	return info
}
