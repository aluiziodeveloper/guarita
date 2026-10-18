package evaluation

import (
	"math"
	"testing"
)

// samples tem quatro ataques e quatro legítimos, com um legítimo de
// pontuação alta e um ataque de pontuação baixa.
var samples = []Sample{
	{true, 0.99},
	{true, 0.9},
	{true, 0.6},
	{true, 0.1},
	{false, 0.7},
	{false, 0.3},
	{false, 0.05},
	{false, 0.01},
}

// #region evaluation-test-at
// No limiar 0,5 há três acertos entre os ataques e um falso positivo:
// precisão 3/4, revocação 3/4, FPR 1/4.
func TestAt(t *testing.T) {
	c := At(samples, 0.5)
	t.Logf("%+v precisão=%.2f revocação=%.2f FPR=%.2f F1=%.2f", c,
		c.Precision(), c.Recall(), c.FPR(), c.F1())
	if c != (Confusion{TP: 3, FP: 1, TN: 3, FN: 1}) ||
		c.Precision() != 0.75 || c.Recall() != 0.75 ||
		c.FPR() != 0.25 || c.F1() != 0.75 {
		t.Errorf("At(0.5) = %+v", c)
	}
}

// #endregion

// #region evaluation-test-choose
// Sem falso positivo nenhum, o melhor é 0,75: dois de quatro ataques.
// Aceitando um em quatro, a revocação sobe para três de quatro, e o
// maior limiar com ela é 0,6.
func TestChoose(t *testing.T) {
	points := Sweep(samples, []float64{0.2, 0.5, 0.6, 0.75, 0.95})
	for _, c := range []struct{ maxFPR, want float64 }{
		{0, 0.75}, {0.25, 0.6},
	} {
		got, ok := Choose(points, c.maxFPR)
		t.Logf("FPR <= %.2f: limiar %.2f, revocação %.2f", c.maxFPR,
			got.Threshold, got.Recall())
		if !ok || got.Threshold != c.want {
			t.Errorf("Choose(%v) = %+v, %v", c.maxFPR, got, ok)
		}
	}
}

// #endregion

func TestChoose_nenhumNoOrcamento(t *testing.T) {
	all := []Sample{{false, 0.9}, {true, 0.95}}
	if p, ok := Choose(Sweep(all, []float64{0.5}), 0); ok {
		t.Errorf("Choose = %+v; nenhum limiar cabe no orçamento", p)
	}
}

func TestRatio_semDenominador(t *testing.T) {
	var c Confusion
	if c.Precision() != 0 || c.Recall() != 0 || c.FPR() != 0 {
		t.Errorf("matriz vazia: %v %v %v", c.Precision(), c.Recall(),
			c.FPR())
	}
}

func TestGrid(t *testing.T) {
	g := Grid(0.05)
	if len(g) != 19 || math.Abs(g[0]-0.05) > 1e-9 ||
		math.Abs(g[18]-0.95) > 1e-9 {
		t.Errorf("Grid(0.05) = %v", g)
	}
}
