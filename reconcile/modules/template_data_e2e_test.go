package modules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestTemplateDataE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		t := g.T()

		data := reconcile.Data("template-e2e-data", func(context.Context) (map[string]string, error) {
			return map[string]string{"Name": "world"}, nil
		}).Node()
		template := modules.TemplateData(input.Static("template-e2e"), input.Static("hello {{.Name}}\n"), data)

		got, err := g.RunNodeErr(g.Context(), template)
		if err != nil {
			t.Fatalf("render template: %v", err)
		}
		if want := []byte("hello world\n"); string(got) != string(want) {
			t.Errorf("rendered template = %q, want %q", got, want)
		}

		sentinel := errors.New("boom")
		badData := reconcile.Data("template-e2e-bad-data", func(context.Context) (string, error) {
			return "", sentinel
		}).Node()
		if _, err := g.RunNodeErr(g.Context(), modules.TemplateData(input.Static("template-e2e-bad"), input.Static("{{.}}"), badData)); !errors.Is(err, sentinel) {
			t.Errorf("template error = %v, want %v", err, sentinel)
		}

		path := g.Path("template-data-file")
		node := modules.File(input.Static(path)).Content(template).Build().Node()
		moduletest.AssertConverges(t, g, node)
		moduletest.AssertFile(t, g, path, []byte("hello world\n"))
	})
}
