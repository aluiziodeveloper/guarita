package approval

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// flood pede a mesma chamada n vezes, cada uma numa goroutine, e
// devolve o canal onde chegam as recusas na entrada da fila.
func flood(t *testing.T, queue *Queue, n int) chan error {
	refused := make(chan error, n)
	for range n {
		go func() {
			if _, err := queue.Await(t.Context(), deleteDB); err != nil {
				refused <- err
			}
		}()
	}
	return refused
}

// #region approval-fatigue-test
// O agente insiste dez vezes: só três pedidos chegam ao operador. Ele
// nega um; a mesma chamada é negada na hora por dez minutos, e depois
// volta a ser aceita.
func TestQueue_limitaAInsistencia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(time.Hour, WithMaxPending(3),
			WithCooldown(10*time.Minute))
		refused := flood(t, queue, 10)
		synctest.Wait() // sete voltaram; três esperam no select
		t.Logf("na fila: %d; recusados na entrada: %d",
			len(queue.Pending()), len(refused))
		_ = queue.Decide(queue.Pending()[0].ID, "ana", false)
		_, err := queue.Await(t.Context(), deleteDB)
		t.Logf("logo depois da negação: %v", err)
		time.Sleep(10 * time.Minute)
		later := queue.wouldAdmit(deleteDB)
		t.Logf("dez minutos depois, entraria? %v", later)
		if len(refused) != 7 || !errors.Is(err, ErrRecentlyDenied) ||
			!later {
			t.Fatal("limites fora do contrato")
		}
	})
}

// #endregion

// wouldAdmit diz se o pedido não seria recusado por repetição.
func (q *Queue) wouldAdmit(input domain.PolicyInput) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	err := q.admit(input, callKey(input))
	return !errors.Is(err, ErrRecentlyDenied)
}
