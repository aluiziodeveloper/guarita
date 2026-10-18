package quota

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// lockedBucket é o mesmo GCRA com uma trava, só para comparação.
type lockedBucket struct {
	mu                  sync.Mutex
	interval, tolerance int64
	epoch               time.Time
	tat                 int64
}

func (b *lockedBucket) Take(n int64) bool {
	now := time.Since(b.epoch).Nanoseconds()
	b.mu.Lock()
	defer b.mu.Unlock()
	next := max(b.tat, now) + n*b.interval
	if next-now > b.tolerance {
		return false
	}
	b.tat = next
	return true
}

// huge é uma ficha por nanossegundo com uma rajada de dias: ela nunca
// acaba durante o benchmark, e mede-se o custo de tirar uma ficha, e
// não o da recusa. Um N maior que 10⁹ por segundo daria um intervalo
// de zero nanossegundo, pela divisão inteira, e o balde não contaria
// nada.
var huge = Rate{N: 1e9, Per: time.Second, Burst: 1 << 50}

// #region quota-benchmark
// BenchmarkBalde tira fichas de um só balde, de todas as goroutines ao
// mesmo tempo: o caso de pior disputa, um agente só chamando uma
// ferramenta só de muitas conexões.
func BenchmarkBalde(b *testing.B) {
	b.Run("atomic", func(b *testing.B) {
		bucket := NewBucket(huge)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bucket.Take(1)
			}
		})
	})
	b.Run("mutex", func(b *testing.B) {
		bucket := &lockedBucket{
			interval: 1, tolerance: huge.Burst, epoch: time.Now(),
		}
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bucket.Take(1)
			}
		})
	})
}

// #endregion

// #region quota-inspector-benchmark
// BenchmarkInspector mede a etapa inteira, com taxa e orçamento, com um
// agente por goroutine, todos ao mesmo tempo.
func BenchmarkInspector(b *testing.B) {
	i := New(Limits{Calls: map[string]Rate{"*": huge}, Budget: &huge})
	var next atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		agent := "agente-" + strconv.FormatInt(next.Add(1), 10)
		input := domain.PolicyInput{
			Agent: agent, Server: "llm",
			Call: domain.Call{ToolName: "ask_model"},
		}
		for pb.Next() {
			_, _ = i.Inspect(b.Context(), input)
		}
	})
}

// #endregion
