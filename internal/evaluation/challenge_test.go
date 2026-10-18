package evaluation

import (
	"math"
	"testing"
)

// Desafio do Capítulo 15: 29 acertos em 30 não quer dizer 97% para
// sempre. Wilson(k, n, z) devolve o intervalo de Wilson para a
// proporção k/n, com z desvios (1,96 para 95%); sem exemplos, o
// intervalo é [0, 1]. O pacote não compila até existir Wilson.
func TestWilson_desafio(t *testing.T) {
	for _, c := range []struct {
		k, n         int
		lower, upper float64
	}{
		{30, 30, 0.8865, 1},      // a revocação do desenvolvimento
		{29, 30, 0.8333, 0.9941}, // a revocação do teste
		{0, 30, 0, 0.1135},
		{9, 12, 0.4677, 0.9111}, // a ofuscação
		{1, 2, 0.0945, 0.9055},
		{0, 0, 0, 1},
	} {
		lower, upper := Wilson(c.k, c.n, 1.96)
		t.Logf("%2d/%2d: [%.4f, %.4f]", c.k, c.n, lower, upper)
		if math.Abs(lower-c.lower) > 1e-4 ||
			math.Abs(upper-c.upper) > 1e-4 {
			t.Errorf("Wilson(%d, %d) = [%.4f, %.4f]; quer [%.4f, %.4f]",
				c.k, c.n, lower, upper, c.lower, c.upper)
		}
	}
}
