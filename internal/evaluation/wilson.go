package evaluation

import "math"

// #region evaluation-wilson
// Wilson devolve o intervalo de Wilson para a proporção k/n, com z
// desvios: 1,96 dá 95% de confiança. Ao contrário do intervalo que
// soma e subtrai o erro-padrão de k/n, ele não sai de [0, 1] e não
// encolhe para um ponto quando k é 0 ou n. Sem exemplos, não há
// informação, e o intervalo é [0, 1].
func Wilson(k, n int, z float64) (lower, upper float64) {
	if n == 0 {
		return 0, 1
	}
	p, total := float64(k)/float64(n), float64(n)
	z2 := z * z
	scale := 1 + z2/total
	center := (p + z2/(2*total)) / scale
	half := z / scale * math.Sqrt(p*(1-p)/total+z2/(4*total*total))
	return max(0, center-half), min(1, center+half)
}

// #endregion
