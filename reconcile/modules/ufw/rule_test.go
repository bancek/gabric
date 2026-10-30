package ufw

import (
	"errors"
	"slices"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func registerUFWHandler(gt *gabrictest.Test, added string) {
	gt.Remote().Handle("ufw", func(cmd gabrictest.Command) gabrictest.Result {
		if slices.Equal(cmd.Args, []string{"show", "added"}) {
			return gabrictest.Result{Stdout: added}
		}

		return gabrictest.Result{}
	})
}

func TestShowAddedData(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWHandler(gt, "ufw allow 22/tcp\n")

	got, err := gt.RunNodeErr(gt.Context(), ShowAddedData())
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "ufw allow 22/tcp\n" {
		t.Errorf("ShowAddedData() = %q, want %q", got, "ufw allow 22/tcp\n")
	}

	gt.Remote().AssertCommands(gt.Command("ufw", "show", "added"))
}

func TestShowAddedDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("ufw", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), ShowAddedData()); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestShowAddedDataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, ShowAddedData()); !errors.Is(err, sentinel) {
		t.Errorf("ShowAddedData() error = %v, want %v", err, sentinel)
	}
}

func TestRuleResourceName(t *testing.T) {
	if got, want := Rule().Port(input.Static("22")).Proto(input.Static("tcp")).Build().Node().Name(), "ufw.rule:22:tcp"; got != want {
		t.Errorf("default name = %q, want %q", got, want)
	}
}

func TestRuleNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWHandler(gt, "ufw allow 22/tcp\n")

	node := Rule().Port(input.Static("22")).Proto(input.Static("tcp")).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("ufw", "show", "added"))
	gt.Remote().AssertCalls()
}

func TestRuleDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWHandler(gt, "\n")

	node := Rule().Port(input.Static("22")).Proto(input.Static("tcp")).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "allow", "22/tcp"),
	)
	gt.Remote().AssertCalls()
}

func TestRuleRoute(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWHandler(gt, "\n")

	node := Rule().Route().Port(input.Static("80")).Proto(input.Static("tcp")).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "route", "allow", "80/tcp"),
	)
	gt.Remote().AssertCalls()
}

func TestRuleOptions(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWHandler(gt, "\n")

	node := Rule().
		Route().
		Action(input.Static("deny")).
		In(input.Static("eth0")).
		Out(input.Static("eth1")).
		From(input.Static("10.0.0.0/8")).
		Port(input.Static("22")).
		Proto(input.Static("tcp")).
		Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "route", "deny", "in", "on", "eth0", "out", "on", "eth1",
			"from", "10.0.0.0/8", "to", "any", "port", "22", "proto", "tcp"),
	)
	gt.Remote().AssertCalls()
}
