// Package evaluation mede um classificador binário num conjunto
// rotulado: a matriz de confusão num limiar, precisão e revocação, a
// varredura de limiares e a escolha de um deles com um critério dito
// antes de olhar os números.
package evaluation

// Sample é um exemplo rotulado com a pontuação do classificador.
type Sample struct {
	Positive bool    // o rótulo: é injeção
	Score    float64 // o que o classificador deu, de 0 a 1
}

// #region evaluation-confusion
// Confusion conta os quatro desfechos num limiar: positivos previstos
// que eram positivos (TP) ou não (FP), negativos previstos que eram
// negativos (TN) ou não (FN).
type Confusion struct{ TP, FP, TN, FN int }

// At classifica cada exemplo no limiar: pontuação maior ou igual a
// ele é positivo previsto, como no inspetor.
func At(samples []Sample, threshold float64) Confusion {
	var c Confusion
	for _, s := range samples {
		switch predicted := s.Score >= threshold; {
		case predicted && s.Positive:
			c.TP++
		case predicted:
			c.FP++
		case s.Positive:
			c.FN++
		default:
			c.TN++
		}
	}
	return c
}

// #endregion

// #region evaluation-metrics
// Precision é a fração dos barrados que eram ataque: TP/(TP+FP).
func (c Confusion) Precision() float64 { return ratio(c.TP, c.TP+c.FP) }

// Recall é a fração dos ataques que foram barrados: TP/(TP+FN).
func (c Confusion) Recall() float64 { return ratio(c.TP, c.TP+c.FN) }

// FPR é a fração dos legítimos barrados: FP/(FP+TN).
func (c Confusion) FPR() float64 { return ratio(c.FP, c.FP+c.TN) }

// F1 é a média harmônica de precisão e revocação.
func (c Confusion) F1() float64 {
	return ratio(2*c.TP, 2*c.TP+c.FP+c.FN)
}

// ratio devolve zero quando não há denominador: sem nenhum barrado, a
// precisão não está definida, e zero não finge que ela é boa.
func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// #endregion

// Point é a matriz de confusão num limiar.
type Point struct {
	Threshold float64
	Confusion
}

// Sweep calcula a matriz em cada limiar dado, na ordem dada.
func Sweep(samples []Sample, thresholds []float64) []Point {
	points := make([]Point, 0, len(thresholds))
	for _, t := range thresholds {
		points = append(points, Point{t, At(samples, t)})
	}
	return points
}

// Grid devolve os limiares de step a 1-step, de step em step.
func Grid(step float64) []float64 {
	var out []float64
	for n := 1; float64(n)*step < 1-step/2; n++ {
		out = append(out, float64(n)*step)
	}
	return out
}

// #region evaluation-choose
// Choose escolhe o limiar com a maior revocação entre os que barram no
// máximo maxFPR dos legítimos; no empate, o maior limiar, que barra
// menos com a mesma revocação. Sem nenhum dentro do orçamento, ok é
// falso: o classificador não serve para esse orçamento.
func Choose(points []Point, maxFPR float64) (best Point, ok bool) {
	for _, p := range points {
		if p.FPR() > maxFPR {
			continue
		}
		recall := p.Recall()
		better := recall > best.Recall() ||
			(recall == best.Recall() && p.Threshold > best.Threshold)
		if !ok || better {
			best, ok = p, true
		}
	}
	return best, ok
}

// #endregion
