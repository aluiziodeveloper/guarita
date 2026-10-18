package metrics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do Capítulo 21: o tempo de cada etapa da linha de inspetores,
// para achar o gargalo pelas métricas, sem o pprof. Inspector e
// ResultInspector envolvem uma etapa com o nome dela e medem cada
// exame num histograma guarita_stage_seconds com o rótulo stage. O
// pacote não compila até existirem os dois métodos.
func TestEtapas_desafio(t *testing.T) {
	m := New()
	fast := m.Inspector("rapida", inspectorFunc(func() {}))
	slow := m.ResultInspector("lenta", resultFunc(func() {
		time.Sleep(2 * time.Millisecond)
	}))
	for range 3 {
		_, _ = fast.Inspect(t.Context(), domain.PolicyInput{})
		_, _ = slow.InspectResult(t.Context(), domain.PolicyInput{}, nil)
	}
	text := scrape(m)
	t.Logf("trecho:\n%s", grep(text, `le="0.001"`, 0, 9))
	for _, line := range []string{
		`guarita_stage_seconds_bucket{stage="rapida",le="0.001"} 3`,
		`guarita_stage_seconds_bucket{stage="lenta",le="0.001"} 0`,
		`guarita_stage_seconds_count{stage="lenta"} 3`,
	} {
		if !strings.Contains(text, line+"\n") {
			t.Errorf("falta %q", line)
		}
	}
}

type inspectorFunc func()

func (f inspectorFunc) Inspect(
	context.Context, domain.PolicyInput,
) (domain.Inspection, error) {
	f()
	return domain.Inspection{Decision: domain.DecisionAllow}, nil
}

type resultFunc func()

func (f resultFunc) InspectResult(
	context.Context, domain.PolicyInput, json.RawMessage,
) (domain.Inspection, error) {
	f()
	return domain.Inspection{Decision: domain.DecisionAllow}, nil
}
