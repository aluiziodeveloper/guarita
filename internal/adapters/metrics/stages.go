package metrics

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region metrics-stage
// stage devolve o histograma da etapa, criado na primeira vez.
func (m *Metrics) stage(name string) *Histogram {
	if h, ok := m.stages.Load(name); ok {
		return h.(*Histogram)
	}
	h := newHistogram("guarita_stage_seconds",
		"Tempo de cada exame de uma etapa de inspeção.")
	h.label = fmt.Sprintf("stage=%q", name)
	actual, _ := m.stages.LoadOrStore(name, h)
	return actual.(*Histogram)
}

// Inspector mede cada exame da etapa de ida, pelo nome dela. É um
// decorador: o caso de uso não sabe que está sendo medido.
func (m *Metrics) Inspector(
	name string, next app.Inspector,
) app.Inspector {
	return timedInspector{next, m.stage(name)}
}

// ResultInspector faz o mesmo com uma etapa de volta.
func (m *Metrics) ResultInspector(
	name string, next app.ResultInspector,
) app.ResultInspector {
	return timedResult{next, m.stage(name)}
}

// #endregion

type timedInspector struct {
	next app.Inspector
	h    *Histogram
}

func (t timedInspector) Inspect(
	ctx context.Context, in domain.PolicyInput,
) (domain.Inspection, error) {
	defer t.h.since(time.Now())
	return t.next.Inspect(ctx, in)
}

type timedResult struct {
	next app.ResultInspector
	h    *Histogram
}

func (t timedResult) InspectResult(
	ctx context.Context, in domain.PolicyInput, result json.RawMessage,
) (domain.Inspection, error) {
	defer t.h.since(time.Now())
	return t.next.InspectResult(ctx, in, result)
}

// since observa o tempo desde start; é o que o defer chama.
func (h *Histogram) since(start time.Time) { h.Observe(time.Since(start)) }

// writeStages escreve a família das etapas, em ordem de nome.
func (m *Metrics) writeStages(w io.Writer) {
	var hs []*Histogram
	m.stages.Range(func(_, v any) bool {
		hs = append(hs, v.(*Histogram))
		return true
	})
	if len(hs) == 0 {
		return
	}
	slices.SortFunc(hs, func(a, b *Histogram) int {
		return cmp.Compare(a.label, b.label)
	})
	hs[0].writeHeader(w)
	for _, h := range hs {
		h.writeSeries(w)
	}
}
