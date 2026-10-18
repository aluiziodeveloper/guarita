package naivequota

import (
	"testing"
	"time"
)

// #region examples-naivequota-test
// Com 60 por minuto, um laço que chama a partir de 10:00:58,5, uma vez
// a cada 25 milissegundos, faz 120 chamadas em três segundos. Todas
// passam: 60 antes da virada do minuto e 60 depois.
func TestWindow_virada(t *testing.T) {
	w := NewWindow(60)
	start := time.Date(2026, 10, 18, 10, 0, 58, 5e8, time.UTC)
	allowed := 0
	for n := range 120 {
		at := start.Add(time.Duration(n) * 25 * time.Millisecond)
		if w.Allow(at) {
			allowed++
		}
	}
	t.Logf("cota de 60/min: %d de 120 chamadas passaram em 3s", allowed)
	if allowed != 120 {
		t.Errorf("o ingênuo mudou de comportamento: %d", allowed)
	}
}

// #endregion
