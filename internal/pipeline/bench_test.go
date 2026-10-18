package pipeline_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/dlp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/injection"
	"github.com/aluiziodeveloper/guarita/internal/adapters/schema"
	"github.com/aluiziodeveloper/guarita/internal/adapters/untrusted"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
	"github.com/aluiziodeveloper/guarita/internal/pipeline"
)

type pinned struct{}

func (pinned) Pinned(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"name":"send_email","inputSchema":{` +
		`"type":"object","additionalProperties":false,"properties":` +
		`{"to":{"type":"string"},"body":{"type":"string"}}}}`), nil
}

type allowAll struct{}

func (allowAll) Evaluate(
	context.Context, domain.PolicyInput,
) (domain.Decision, error) {
	return domain.DecisionAllow, nil
}

// Um corpo e um resultado de 1 KiB de texto legítimo: o caminho comum,
// em que toda etapa examina tudo e nenhuma para a fila.
var (
	text      = strings.Repeat("Vendas estáveis no trimestre. ", 35)[:1024]
	arguments = json.RawMessage(`{"to":"time@lab.test","body":"` + text +
		`"}`)
	result = json.RawMessage(`{"content":[{"type":"text","text":"` +
		text + `"}]}`)
	call = domain.Call{ToolName: "send_email", Arguments: arguments}
)

func stages() pipeline.Registry {
	stage := func(s pipeline.Stage) pipeline.Factory {
		return func() (pipeline.Stage, error) { return s, nil }
	}
	return pipeline.Registry{
		"schema": stage(pipeline.Stage{Call: schema.New(pinned{})}),
		"dlp": stage(pipeline.Stage{Call: dlp.New(dlp.DefaultActions(),
			4.3, dlp.WithAllowedHosts("lab.test"))}),
		"injection": stage(pipeline.Stage{
			Result: injection.New(injection.DefaultActions()),
		}),
		"mark": stage(pipeline.Stage{Marker: untrusted.New()}),
	}
}

// #region pipeline-benchmark
// BenchmarkLinha mede o caso de uso com cada linha: nenhuma etapa,
// cada etapa sozinha e as linhas completas das duas rotas. A ida passa
// por Authorize; a volta, por ReviewResult.
func BenchmarkLinha(b *testing.B) {
	for _, names := range [][]string{
		{},
		{"schema"},
		{"dlp"},
		{"schema", "dlp"},
		{"injection"},
		{"mark"},
		{"injection", "mark"},
	} {
		line, err := stages().Build(names)
		if err != nil {
			b.Fatal(err)
		}
		uc := app.NewInterceptCall(allowAll{}, nil, line.Options()...)
		b.Run(label(names), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = uc.Authorize(b.Context(), call)
				_, _ = uc.ReviewResult(b.Context(), call, result)
			}
		})
	}
}

// #endregion

// label dá nome ao subteste: as etapas, ou "vazia".
func label(names []string) string {
	if len(names) == 0 {
		return "vazia"
	}
	return strings.Join(names, "+")
}
