package modules_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestFileE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		t := g.T()
		path := g.Path("app.conf")

		file := modules.File(input.Static(path)).Content(input.Static([]byte("hello\n")))
		if g.POSIXModes {
			file = file.Mode(modules.FileMode(0o640))
		}
		if g.Root {
			file = file.User(input.Static("root")).Group(input.Static("root"))
		}

		node := file.Build().Node()

		moduletest.AssertDryRun(t, g, node)
		moduletest.AssertConverges(t, g, node)
		moduletest.AssertFile(t, g, path, []byte("hello\n"))

		if g.POSIXModes {
			stat, err := g.Ops().Stat(path).RunErr()
			if err != nil {
				t.Fatalf("stat %s: %v", path, err)
			}
			if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
				t.Errorf("mode = %v, want %v", got, want)
			}
			if g.Root && (stat.User != "root" || stat.Group != "root") {
				t.Errorf("owner = %s:%s, want root:root", stat.User, stat.Group)
			}
		}

		createOnlyPath := g.Path("create-only.conf")
		if err := g.Ops().Put(createOnlyPath).EnsureParent().String("keep").RunErr(); err != nil {
			t.Fatalf("write %s: %v", createOnlyPath, err)
		}

		createOnly := modules.File(input.Static(createOnlyPath)).
			Content(input.Static([]byte("new"))).
			CreateOnly().
			Build().
			Node()

		changed, err := g.RunNodeErr(g.Context(), createOnly)
		if err != nil {
			t.Fatalf("create-only: %v", err)
		}
		if changed {
			t.Error("CreateOnly changed an existing file")
		}
		moduletest.AssertFile(t, g, createOnlyPath, []byte("keep"))

		nestedPath := g.Path("nested/deep/app.conf")
		nested := modules.File(input.Static(nestedPath)).Content(input.Static([]byte("nested\n"))).Build().Node()
		moduletest.AssertConverges(t, g, nested)
		moduletest.AssertFile(t, g, nestedPath, []byte("nested\n"))

		funcPath := g.Path("content-func.conf")
		content := reconcile.Data("file-e2e-content", func(context.Context) ([]byte, error) {
			return []byte("func\n"), nil
		}).Node()
		funcFile := modules.File(input.Static(funcPath)).Content(content).Build().Node()
		moduletest.AssertConverges(t, g, funcFile)
		moduletest.AssertFile(t, g, funcPath, []byte("func\n"))

		templatePath := g.Path("template.conf")
		templateData := reconcile.Data("file-e2e-template", func(context.Context) (string, error) {
			return "world", nil
		}).Node()
		template := modules.TemplateData(input.Static(templatePath), input.Static("hello {{.}}\n"), templateData)
		templateFile := modules.File(input.Static(templatePath)).Content(template).Build().Node()
		moduletest.AssertConverges(t, g, templateFile)
		moduletest.AssertFile(t, g, templatePath, []byte("hello world\n"))
	})
}
