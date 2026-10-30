package modules

import (
	"bytes"
	"context"
	"text/template"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// TemplateData renders tmpl with data and yields the rendered bytes as a data
// node. The template text is read and parsed once per run; an invalid template
// returns an error. The node is named "modules.template-data:<path>" and is
// memoized once per run.
func TemplateData[T any](
	pathInput input.Input[string],
	tmplInput input.Input[string],
	dataInput input.Input[T],
) *reconcile.Node[[]byte] {
	if pathInput == nil {
		panic("modules: TemplateData: nil path")
	}
	if tmplInput == nil {
		panic("modules: TemplateData: nil template")
	}
	if dataInput == nil {
		panic("modules: TemplateData: nil data")
	}

	return reconcile.Data("modules.template-data:"+pathInput.Name(), func(ctx context.Context) ([]byte, error) {
		tmpl, err := tmplInput.Value(ctx)
		if err != nil {
			return nil, err
		}

		t, err := template.New("file").Parse(tmpl)
		if err != nil {
			return nil, err
		}

		data, err := dataInput.Value(ctx)
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	}).Node()
}
