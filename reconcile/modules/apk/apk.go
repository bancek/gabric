package apk

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// PackageBuilder builds a node that ensures an apk package is installed.
type PackageBuilder struct {
	nameInput   input.Input[string]
	latestInput input.Input[bool]
}

// Package ensures the apk package name is installed. The node's value reports
// whether it changed.
func Package(nameInput input.Input[string]) *PackageBuilder {
	return &PackageBuilder{nameInput: nameInput}
}

// Latest also upgrades the package when the current apk candidate is newer. It
// compares the apk candidate; it does not run apk update (make a Command node
// for that a dependency, with Needs or CheckNeeds).
func (b *PackageBuilder) Latest(flag ...input.Input[bool]) *PackageBuilder {
	b.latestInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile node builder.
func (b *PackageBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("apk: Package: nil name")
	}

	nameInput, latestInput := b.nameInput, b.latestInput

	return reconcile.ResourceBool(
		"apk.package:"+nameInput.Name(),
		func(ctx context.Context) (bool, error) {
			return SatisfiedData(nameInput, latestInput).Value(ctx)
		},
		func(ctx context.Context) error {
			name, err := nameInput.Value(ctx)
			if err != nil {
				return err
			}
			latest, err := input.Value(ctx, latestInput)
			if err != nil {
				return err
			}

			cmd := []string{"apk", "add"}
			if latest {
				cmd = append(cmd, "-u")
			}
			cmd = append(cmd, name)

			return gabric.TargetOps(ctx).Exec(cmd...).RunErr()
		})
}
