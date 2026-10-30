package pkg

import (
	"context"
	"fmt"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/apk"
	"github.com/bancek/gabric/reconcile/modules/apt"
)

// PackageBuilder builds a node that ensures a package is installed using the
// target's package manager.
type PackageBuilder struct {
	nameInput   input.Input[string]
	latestInput input.Input[bool]
}

// Package ensures the package name is installed. The node's value reports
// whether it changed.
func Package(nameInput input.Input[string]) *PackageBuilder {
	return &PackageBuilder{nameInput: nameInput}
}

// Latest also upgrades the package when the current candidate is newer.
func (b *PackageBuilder) Latest(flag ...input.Input[bool]) *PackageBuilder {
	b.latestInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile resource builder.
func (b *PackageBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.nameInput == nil {
		panic("pkg: Package: nil name")
	}

	nameInput, latestInput := b.nameInput, b.latestInput
	managerData := ManagerData()

	return reconcile.DynamicResource("pkg.package:"+nameInput.Name(), func(ctx context.Context) (*reconcile.ResourceBuilder[bool], error) {
		manager, err := managerData.Value(ctx)
		if err != nil {
			return nil, err
		}

		switch manager {
		case ManagerApt:
			p := apt.Package(nameInput)
			if latestInput != nil {
				p = p.Latest(latestInput)
			}
			return p.Build(), nil
		case ManagerApk:
			p := apk.Package(nameInput)
			if latestInput != nil {
				p = p.Latest(latestInput)
			}
			return p.Build(), nil
		default:
			return nil, fmt.Errorf("pkg: unsupported package manager")
		}
	})
}
