package ufw

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// RuleBuilder builds a node that ensures a UFW rule exists.
type RuleBuilder struct {
	portInput   input.Input[string]
	protoInput  input.Input[string]
	actionInput input.Input[string]
	fromInput   input.Input[string]
	inInput     input.Input[string]
	outInput    input.Input[string]
	routeInput  input.Input[bool]
}

// Rule ensures a UFW rule exists. The node yields bool (changed) and does
// not notify by itself.
func Rule() *RuleBuilder {
	return &RuleBuilder{}
}

// Port sets the rule port.
func (b *RuleBuilder) Port(portInput input.Input[string]) *RuleBuilder {
	b.portInput = portInput
	return b
}

// Proto sets the rule protocol (tcp, udp).
func (b *RuleBuilder) Proto(protoInput input.Input[string]) *RuleBuilder {
	b.protoInput = protoInput
	return b
}

// Action sets the rule action (default "allow").
func (b *RuleBuilder) Action(actionInput input.Input[string]) *RuleBuilder {
	b.actionInput = actionInput
	return b
}

// From sets the source address (with Port).
func (b *RuleBuilder) From(fromInput input.Input[string]) *RuleBuilder {
	b.fromInput = fromInput
	return b
}

// In sets the incoming interface ("in on").
func (b *RuleBuilder) In(ifaceInput input.Input[string]) *RuleBuilder {
	b.inInput = ifaceInput
	return b
}

// Out sets the outgoing interface ("out on").
func (b *RuleBuilder) Out(ifaceInput input.Input[string]) *RuleBuilder {
	b.outInput = ifaceInput
	return b
}

// Route makes the rule a route rule.
func (b *RuleBuilder) Route(flag ...input.Input[bool]) *RuleBuilder {
	b.routeInput = input.Flag(flag...)
	return b
}

// ShowAddedData reads `ufw show added` as a data node (dry-run safe,
// memoized once per run). Errors propagate.
func ShowAddedData() *reconcile.Node[string] {
	return reconcile.Data("ufw.show-added", func(ctx context.Context) (string, error) {
		return modules.CommandOutputData(input.List("ufw", "show", "added")).Value(ctx)
	}).Node()
}

type ruleSpec struct {
	port   string
	proto  string
	action string
	from   string
	in     string
	out    string
	route  bool
}

func (b *RuleBuilder) resolve(ctx context.Context) (ruleSpec, error) {
	port, err := input.Value(ctx, b.portInput)
	if err != nil {
		return ruleSpec{}, err
	}
	proto, err := input.Value(ctx, b.protoInput)
	if err != nil {
		return ruleSpec{}, err
	}
	action, err := input.Value(ctx, b.actionInput)
	if err != nil {
		return ruleSpec{}, err
	}
	from, err := input.Value(ctx, b.fromInput)
	if err != nil {
		return ruleSpec{}, err
	}
	in, err := input.Value(ctx, b.inInput)
	if err != nil {
		return ruleSpec{}, err
	}
	out, err := input.Value(ctx, b.outInput)
	if err != nil {
		return ruleSpec{}, err
	}
	route, err := input.Value(ctx, b.routeInput)
	if err != nil {
		return ruleSpec{}, err
	}

	return ruleSpec{
		port:   port,
		proto:  proto,
		action: action,
		from:   from,
		in:     in,
		out:    out,
		route:  route,
	}, nil
}

func (b *RuleBuilder) resourceName() string {
	var parts []string
	if b.routeInput != nil {
		parts = append(parts, "route")
	}
	if b.actionInput != nil {
		parts = append(parts, b.actionInput.Name())
	}
	if b.inInput != nil {
		parts = append(parts, "in:"+b.inInput.Name())
	}
	if b.outInput != nil {
		parts = append(parts, "out:"+b.outInput.Name())
	}
	if b.portInput != nil {
		parts = append(parts, b.portInput.Name())
	}
	if b.protoInput != nil {
		parts = append(parts, b.protoInput.Name())
	}
	if len(parts) == 0 {
		return "ufw.rule"
	}

	return "ufw.rule:" + strings.Join(parts, ":")
}

// Build hands off to the reconcile node builder. The canonical command matches
// what `ufw show added` prints.
func (b *RuleBuilder) Build() *reconcile.ResourceBuilder[bool] {
	return reconcile.ResourceBool(b.resourceName(),
		func(ctx context.Context) (bool, error) {
			spec, err := b.resolve(ctx)
			if err != nil {
				return false, err
			}
			command := strings.Join(spec.argv(), " ")

			out, err := ShowAddedData().Value(ctx)
			if err != nil {
				return false, err
			}

			for line := range strings.SplitSeq(out, "\n") {
				if strings.TrimSpace(line) == command {
					return true, nil
				}
			}

			return false, nil
		},
		func(ctx context.Context) error {
			spec, err := b.resolve(ctx)
			if err != nil {
				return err
			}

			return gabric.TargetOps(ctx).Exec(spec.argv()...).RunErr()
		})
}

func (s ruleSpec) actionOrDefault() string {
	if s.action == "" {
		return "allow"
	}

	return s.action
}

func (s ruleSpec) argv() []string {
	parts := []string{"ufw"}

	if s.route {
		parts = append(parts, "route")
	}

	parts = append(parts, s.actionOrDefault())

	if s.in != "" {
		parts = append(parts, "in", "on", s.in)
	}

	if s.out != "" {
		parts = append(parts, "out", "on", s.out)
	}

	if s.port != "" {
		spec := s.port
		if s.proto != "" {
			spec += "/" + s.proto
		}

		if s.from != "" {
			parts = append(parts, "from", s.from, "to", "any", "port", s.port)

			if s.proto != "" {
				parts = append(parts, "proto", s.proto)
			}
		} else {
			parts = append(parts, spec)
		}
	}

	return parts
}
