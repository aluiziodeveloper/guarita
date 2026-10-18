package approval

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do Capítulo 12: delete_database exige dois operadores
// diferentes. O primeiro "aprovar" registra a assinatura e deixa o
// pedido pendente; o mesmo operador de novo é ErrAlreadySigned; o
// segundo aprova. Uma negação vale sozinha, e as outras ferramentas
// continuam precisando de um operador só.
func TestQueue_desafioDoisOperadores(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(time.Hour, WithQuorum("delete_database", 2))
		result := make(chan domain.ApprovalState, 1)
		go func() {
			state, _ := queue.Await(t.Context(), deleteDB)
			result <- state
		}()
		synctest.Wait()
		id := queue.Pending()[0].ID
		if err := queue.Decide(id, "ana", true); err != nil {
			t.Fatalf("primeira assinatura: %v", err)
		}
		if len(queue.Pending()) != 1 {
			t.Fatal("uma assinatura não basta para delete_database")
		}
		if err := queue.Decide(id, "ana", true); !errors.Is(err,
			ErrAlreadySigned) {
			t.Fatalf("a mesma operadora de novo: %v", err)
		}
		if err := queue.Decide(id, "bruno", true); err != nil {
			t.Fatalf("segunda assinatura: %v", err)
		}
		if state := <-result; state != domain.ApprovalApproved {
			t.Fatalf("estado %s", state)
		}
	})
}

func TestQueue_desafioNegacaoValeSozinha(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(time.Hour, WithQuorum("delete_database", 2))
		result := make(chan domain.ApprovalState, 1)
		go func() {
			state, _ := queue.Await(t.Context(), deleteDB)
			result <- state
		}()
		synctest.Wait()
		id := queue.Pending()[0].ID
		_ = queue.Decide(id, "ana", true)
		_ = queue.Decide(id, "bruno", false)
		if state := <-result; state != domain.ApprovalDenied {
			t.Fatalf("estado %s", state)
		}
	})
}

func TestQueue_desafioOutrasFerramentasUmOperador(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := New(time.Hour, WithQuorum("delete_database", 2))
		purge := deleteDB
		purge.Call.ToolName = "purge_records"
		result := make(chan domain.ApprovalState, 1)
		go func() {
			state, _ := queue.Await(t.Context(), purge)
			result <- state
		}()
		synctest.Wait()
		_ = queue.Decide(queue.Pending()[0].ID, "ana", true)
		if state := <-result; state != domain.ApprovalApproved {
			t.Fatalf("estado %s", state)
		}
	})
}
