package quota

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func rate(t *testing.T, text string) Rate {
	t.Helper()
	r, err := ParseRate(text)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// #region quota-bucket-test
// A mesma cena do ingênuo: 60/min, 120 chamadas a cada 25 ms. O balde
// deixa passar a rajada de 60 e as duas fichas repostas, uma por
// segundo, durante os três segundos. No relógio falso de synctest, os
// três segundos passam na hora.
func TestBucket_rajada(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBucket(rate(t, "60/m"))
		allowed, wait := 0, time.Duration(0)
		for range 120 {
			if ok, w := b.Take(1); ok {
				allowed++
			} else {
				wait = w
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Logf("60/m: %d de 120 em 3s; a última recusa pedia %v",
			allowed, wait)
		if allowed != 62 {
			t.Errorf("passaram %d", allowed)
		}
	})
}

// #endregion

// #region quota-bucket-concurrency-test
// 64 goroutines tiram fichas ao mesmo tempo de um balde de 1000: passam
// exatamente 1000, nem uma a mais. Sem o compare-and-swap, duas
// goroutines tirariam a mesma ficha.
func TestBucket_concorrencia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBucket(rate(t, "1000/h"))
		var allowed atomic.Int64
		var wg sync.WaitGroup
		for range 64 {
			wg.Go(func() {
				for range 100 {
					if ok, _ := b.Take(1); ok {
						allowed.Add(1)
					}
				}
			})
		}
		wg.Wait()
		t.Logf("64 goroutines × 100: %d passaram", allowed.Load())
		if allowed.Load() != 1000 {
			t.Errorf("passaram %d", allowed.Load())
		}
	})
}

// #endregion

func TestParseRate(t *testing.T) {
	for _, bad := range []string{"", "60", "60/d", "0/m", "x/m", "6/m:0"} {
		if _, err := ParseRate(bad); !errors.Is(err, ErrRate) {
			t.Errorf("ParseRate(%q) = %v", bad, err)
		}
	}
	r, err := ParseRate("60/m:10")
	if err != nil || r != (Rate{N: 60, Per: time.Minute, Burst: 10}) {
		t.Errorf("ParseRate = %+v, %v", r, err)
	}
}

func TestBucket_custo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBucket(rate(t, "100/h"))
		ok1, _ := b.Take(60)
		ok2, wait := b.Take(60)
		if !ok1 || ok2 || wait != 20*36*time.Second {
			t.Errorf("custo: %v %v %v", ok1, ok2, wait)
		}
	})
}
