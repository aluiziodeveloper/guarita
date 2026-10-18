// Package metrics mede a Guarita em operação e expõe as medidas no
// formato de texto do Prometheus: quanto tempo cada pedido levou,
// quanto disso foi o servidor, quanto foi a própria Guarita, e quantas
// chamadas foram repassadas ou negadas. Nenhuma medida leva argumentos,
// nomes de ferramentas ou identidades: só tempos e contagens.
package metrics

import (
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"
)

// Buckets são os limites dos histogramas, em segundos: de 50 µs, onde
// fica uma decisão sem inspetores, até 5 s, onde fica o classificador.
var Buckets = []float64{
	0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01,
	0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
}

// #region metrics-histogram
// Histogram conta as observações por faixa de tempo, sem trava: cada
// faixa é um contador atômico, como o balde do Capítulo 18. A soma vai
// em nanossegundos, um inteiro, para também ser atômica.
type Histogram struct {
	name, help string
	label      string          // como stage="dlp", ou vazio
	counts     []atomic.Uint64 // uma por limite, e a última para +Inf
	sum        atomic.Int64
}

func newHistogram(name, help string) *Histogram {
	return &Histogram{
		name: name, help: help,
		counts: make([]atomic.Uint64, len(Buckets)+1),
	}
}

// Observe conta d na primeira faixa que a comporta.
func (h *Histogram) Observe(d time.Duration) {
	i := 0
	for i < len(Buckets) && d.Seconds() > Buckets[i] {
		i++
	}
	h.counts[i].Add(1)
	h.sum.Add(int64(d))
}

// #endregion

// #region metrics-histogram-write
// write escreve o histograma no formato do Prometheus: cada faixa é
// cumulativa, com todas as observações até o limite dela.
func (h *Histogram) write(w io.Writer) {
	h.writeHeader(w)
	h.writeSeries(w)
}

// #endregion

// writeHeader escreve a ajuda e o tipo, uma vez por família.
func (h *Histogram) writeHeader(w io.Writer) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n",
		h.name, h.help, h.name)
}

// #region metrics-histogram-series
// writeSeries escreve as faixas, a soma e a contagem, com o rótulo.
func (h *Histogram) writeSeries(w io.Writer) {
	prefix, labels := "", ""
	if h.label != "" {
		prefix, labels = h.label+",", "{"+h.label+"}"
	}
	var total uint64
	for i := range h.counts {
		total += h.counts[i].Load()
		le := "+Inf"
		if i < len(Buckets) {
			le = strconv.FormatFloat(Buckets[i], 'g', -1, 64)
		}
		_, _ = fmt.Fprintf(w, "%s_bucket{%sle=%q} %d\n", h.name, prefix,
			le, total)
	}
	sum := time.Duration(h.sum.Load()).Seconds()
	_, _ = fmt.Fprintf(w, "%s_sum%s %s\n%s_count%s %d\n", h.name,
		labels, strconv.FormatFloat(sum, 'g', -1, 64), h.name, labels,
		total)
}

// #endregion

// counter é um contador com um rótulo de valor fixo.
type counter struct {
	label string
	value atomic.Uint64
}
