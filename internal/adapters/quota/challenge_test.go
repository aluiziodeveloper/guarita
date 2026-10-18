package quota

import (
	"testing"
	"testing/synctest"
)

// Desafio do Capítulo 18: uma chamada barrada pelo orçamento não devia
// gastar a cota de chamadas. Bucket.Give(n) devolve n fichas ao balde,
// sem passar de cheio, com compare-and-swap, e o inspetor devolve a
// ficha da cota quando o orçamento barra a chamada. O pacote não
// compila até existir Give.
func TestGive_desafio(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBucket(rate(t, "5/m"))
		for range 5 {
			b.Take(1)
		}
		b.Give(2)
		ok1, _ := b.Take(2)
		ok2, _ := b.Take(1)
		b.Give(100) // não passa de cheio: cabem 5
		full := 0
		for range 6 {
			if ok, _ := b.Take(1); ok {
				full++
			}
		}
		t.Logf("devolvidas 2: %v %v; cheio de novo: %d", ok1, ok2, full)
		if !ok1 || ok2 || full != 5 {
			t.Errorf("Give: %v %v %d", ok1, ok2, full)
		}
	})
}

func TestInspect_desafioDevolveAFicha(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		i := New(Limits{
			Calls:  map[string]Rate{"*": rate(t, "60/m")},
			Budget: new(rate(t, "10/h")),
		})
		got, findings := count(t, i, "laco", "web_search", 61)
		t.Logf("passaram %d; último achado %v", got, findings)
		if got != 10 || findings[0].Signal != SignalBudget {
			t.Errorf("passaram %d; último achado %v", got, findings)
		}
	})
}
