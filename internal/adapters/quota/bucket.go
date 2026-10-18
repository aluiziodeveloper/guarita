// Package quota limita quantas chamadas cada agente faz a cada
// ferramenta e quanto custo ele consome, com baldes de fichas.
package quota

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ErrRate indica uma taxa mal escrita.
var ErrRate = errors.New("quota: rate must be N/s, N/m or N/h, with :burst")

// Rate é uma taxa: N fichas a cada Per, com até Burst de uma vez.
type Rate struct {
	N     int64
	Per   time.Duration
	Burst int64
}

// #region quota-parse-rate
// ParseRate lê "60/m" ou "60/m:10". Sem rajada, ela é N: o agente pode
// gastar a cota de um período de uma vez, e depois espera a reposição.
func ParseRate(text string) (Rate, error) {
	spec, burst, hasBurst := strings.Cut(text, ":")
	count, unit, ok := strings.Cut(spec, "/")
	n, err := strconv.ParseInt(count, 10, 64)
	per, known := map[string]time.Duration{
		"s": time.Second, "m": time.Minute, "h": time.Hour,
	}[unit]
	if !ok || err != nil || n <= 0 || !known {
		return Rate{}, fmt.Errorf("%w: %q", ErrRate, text)
	}
	r := Rate{N: n, Per: per, Burst: n}
	if hasBurst {
		r.Burst, err = strconv.ParseInt(burst, 10, 64)
		if err != nil || r.Burst <= 0 {
			return Rate{}, fmt.Errorf("%w: %q", ErrRate, text)
		}
	}
	return r, nil
}

// #endregion

// #region quota-bucket
// Bucket é um balde de fichas pelo algoritmo GCRA: em vez de contar
// fichas e reabastecer, ele guarda um só número, o instante teórico
// (tat) em que o balde estaria cheio de novo. Tirar n fichas empurra
// esse instante n intervalos para a frente; se ele passar da
// tolerância, que é a rajada, não há fichas.
type Bucket struct {
	interval  int64 // nanossegundos por ficha
	tolerance int64 // rajada × intervalo
	epoch     time.Time
	tat       atomic.Int64 // nanossegundos desde epoch
}

// NewBucket cria o balde cheio.
func NewBucket(r Rate) *Bucket {
	interval := int64(r.Per) / r.N
	return &Bucket{
		interval: interval, tolerance: r.Burst * interval,
		epoch: time.Now(),
	}
}

// #endregion

// #region quota-take
// Take tira n fichas, se houver, e devolve quanto esperar se não
// houver. A atualização é um compare-and-swap: duas goroutines que
// leram o mesmo tat não tiram as mesmas fichas, porque só uma troca o
// valor, e a outra lê de novo e refaz a conta. time.Since usa o
// relógio monotônico, que não volta quando o relógio do sistema é
// acertado.
func (b *Bucket) Take(n int64) (bool, time.Duration) {
	now := time.Since(b.epoch).Nanoseconds()
	for {
		old := b.tat.Load()
		next := max(old, now) + n*b.interval
		if over := next - now - b.tolerance; over > 0 {
			return false, time.Duration(over)
		}
		if b.tat.CompareAndSwap(old, next) {
			return true, 0
		}
	}
}

// #endregion
