package modules

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const templatePath = "/srv/c.yaml"

func templatePutCall(content string, mode fs.FileMode) gabrictest.Call {
	return gabrictest.Call{Put: &gabric.PutRequest{
		Path:         templatePath,
		Content:      bytes.NewBufferString(content),
		Mode:         mode,
		EnsureParent: true,
	}}
}

type templateTestData struct {
	IPv6 bool
}

func TestTemplateData(t *testing.T) {
	data := reconcile.Data("data", func(context.Context) (templateTestData, error) {
		return templateTestData{IPv6: true}, nil
	}).Node()

	node := TemplateData(input.Static(templatePath), input.Static("ipv6: {{.IPv6}}\n"), data)

	if got, want := node.Name(), "modules.template-data:"+templatePath; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	gt := gabrictest.New(t)

	if got, want := string(gt.RunNode(gt.Context(), node)), "ipv6: true\n"; got != want {
		t.Errorf("TemplateData() = %q, want %q", got, want)
	}
}

func TestTemplateDataError(t *testing.T) {
	data := reconcile.Data("data", func(context.Context) (any, error) {
		return nil, errors.New("data failed")
	}).Node()

	gt := gabrictest.New(t)

	if _, err := gt.RunNodeErr(gt.Context(), TemplateData(input.Static(templatePath), input.Static("{{.}}"), data)); err == nil {
		t.Error("TemplateData() error = nil, want error")
	}
}

func TestTemplateDataFileMatches(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(templatePath, []byte("ipv6: true\n"), 0o644)

	data := reconcile.Data("data", func(context.Context) (map[string]bool, error) {
		return map[string]bool{"IPv6": true}, nil
	}).Node()

	node := File(input.Static(templatePath)).
		Content(TemplateData(input.Static(templatePath), input.Static("ipv6: {{.IPv6}}\n"), data)).
		Mode(FileMode(0o644)).
		Build().
		Node()

	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", templatePath))
	gt.Remote().AssertCalls(statCall(templatePath))
}

func TestTemplateDataFileDrift(t *testing.T) {
	gt := gabrictest.New(t)

	data := reconcile.Data("data", func(context.Context) (map[string]bool, error) {
		return map[string]bool{"IPv6": true}, nil
	}).Node()

	node := File(input.Static(templatePath)).
		Content(TemplateData(input.Static(templatePath), input.Static("ipv6: {{.IPv6}}\n"), data)).
		Mode(FileMode(0o644)).
		Build().
		Node()

	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", templatePath))
	gt.Remote().AssertCalls(
		existsCall(templatePath, gabric.FileKindAny),
		templatePutCall("ipv6: true\n", 0o644),
	)
}

func TestTemplateDataMemoized(t *testing.T) {
	gt := gabrictest.New(t)

	calls := 0

	data := reconcile.Data("data", func(context.Context) (any, error) {
		calls++
		return map[string]bool{"IPv6": true}, nil
	}).Node()

	node := File(input.Static(templatePath)).
		Content(TemplateData(input.Static(templatePath), input.Static("ipv6: {{.IPv6}}\n"), data)).
		Build().
		Node()

	gt.RunNode(gt.Context(), node)

	if calls != 1 {
		t.Errorf("TemplateData data calls = %d, want 1 (memoized per run)", calls)
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", templatePath))
	gt.Remote().AssertCalls(
		existsCall(templatePath, gabric.FileKindAny),
		templatePutCall("ipv6: true\n", 0),
	)
}

func TestTemplateDataOwner(t *testing.T) {
	gt := gabrictest.New(t)

	data := reconcile.Data("data", func(context.Context) (any, error) {
		return map[string]bool{"IPv6": true}, nil
	}).Node()

	node := File(input.Static(templatePath)).
		Content(TemplateData(input.Static(templatePath), input.Static("ipv6: {{.IPv6}}\n"), data)).
		User(input.Static("root")).
		Group(input.Static("root")).
		Mode(FileMode(0o644)).
		Build().
		Node()

	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", templatePath))
	gt.Remote().AssertCalls(
		existsCall(templatePath, gabric.FileKindAny),
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         templatePath,
			Content:      bytes.NewBufferString("ipv6: true\n"),
			Mode:         0o644,
			User:         "root",
			Group:        "root",
			EnsureParent: true,
		}},
	)
}

func TestTemplateDataInvalidTemplate(t *testing.T) {
	gt := gabrictest.New(t)
	node := TemplateData[any](
		input.Static(templatePath),
		input.Static("{{.Broken"),
		input.Static[any](nil),
	)

	if _, err := gt.RunNodeErr(gt.Context(), node); err == nil {
		t.Error("TemplateData() error = nil, want error")
	}
}

func TestTemplateDataDynamicTemplate(t *testing.T) {
	tmpl := reconcile.Data("template", func(context.Context) (string, error) {
		return "hello {{.}}\n", nil
	}).Node()
	data := input.Static("world")

	gt := gabrictest.New(t)
	node := TemplateData(input.Static(templatePath), tmpl, data)

	if got, want := string(gt.RunNode(gt.Context(), node)), "hello world\n"; got != want {
		t.Errorf("TemplateData() = %q, want %q", got, want)
	}
}
