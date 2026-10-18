package approval

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var deleteDB = domain.PolicyInput{
	Agent: "manutencao", Server: "db",
	Call: domain.Call{ToolName: "delete_database"},
}

// #region approval-timeout-test
// Dentro de synctest.Test, o relógio é falso: ele só anda quando todas
// as goroutines da bolha estão bloqueadas. Dois minutos passam na hora.
func TestQueue_expiraNoPrazo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(2 * time.Minute)
		start := time.Now()
		state, err := queue.Await(t.Context(), deleteDB)
		t.Logf("estado %s depois de %v", state, time.Since(start))
		if err != nil || state != domain.ApprovalExpired ||
			time.Since(start) != 2*time.Minute {
			t.Fatalf("%s, %v", state, err)
		}
	})
}

// #endregion

// #region approval-decide-test
// O agente espera; synctest.Wait devolve quando ele está bloqueado no
// select; o operador aprova, e a espera termina sem o relógio andar.
func TestQueue_operadorDecide(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(2 * time.Minute)
		result := make(chan domain.ApprovalState)
		go func() {
			state, _ := queue.Await(t.Context(), deleteDB)
			result <- state
		}()
		synctest.Wait()
		id := queue.Pending()[0].ID
		self := queue.Decide(id, "manutencao", true)
		err := queue.Decide(id, "ana", true)
		state := <-result
		replay := queue.Decide(id, "ana", false)
		t.Logf("id %s; o próprio agente: %v", id, self)
		t.Logf("ana: %v -> %s; de novo: %v", err, state, replay)
		if !errors.Is(self, ErrSelfApproval) || err != nil ||
			state != domain.ApprovalApproved ||
			!errors.Is(replay, domain.ErrInvalidTransition) {
			t.Fatal("decisão fora do contrato")
		}
	})
}

// #endregion

func TestQueue_agenteDesiste(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(time.Minute)
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(10*time.Second, cancel)
		state, _ := queue.Await(ctx, deleteDB)
		if state != domain.ApprovalCanceled || len(queue.Pending()) != 0 {
			t.Fatalf("estado %s", state)
		}
		if err := queue.Decide("nao-existe", "ana", true); !errors.Is(err,
			ErrUnknown) {
			t.Fatalf("Decide() = %v", err)
		}
	})
}
