package approvalfatigue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/approval"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var deleteDB = domain.Call{
	ToolName: "delete_database", Arguments: json.RawMessage(`{}`),
}

// guarita monta o caso de uso com a política que pede aprovação para
// delete_database e a fila dada.
func guarita(queue *approval.Queue) *app.InterceptCall {
	policy, _ := policystatic.New(map[string]domain.Decision{
		"delete_database": domain.DecisionRequireApproval,
	})
	agent := func(context.Context) string { return "manutencao" }
	return app.NewInterceptCall(policy, nil,
		app.WithApprover(queue), app.WithAgent(agent))
}

// queues são as duas filas do ataque: a sem limites e a da Guarita.
var queues = []struct {
	name  string
	queue func() *approval.Queue
}{
	{"sem limites", func() *approval.Queue {
		return approval.New(time.Hour)
	}},
	{"com limites", func() *approval.Queue {
		return approval.New(time.Hour, approval.WithMaxPending(3),
			approval.WithCooldown(10*time.Minute))
	}},
}

// #region attacks-fatigue-test
// O agente pede vinte vezes; o operador nega o primeiro pedido que vê,
// e o agente pede de novo. Sem limites, os vinte chegam ao operador e
// a repetição volta à fila. Com eles, três chegam e a repetição é
// negada sem chegar a ninguém.
func TestFatigue(t *testing.T) {
	for _, c := range queues {
		synctest.Test(t, func(t *testing.T) {
			queue := c.queue()
			seen := insist(t, queue, 20)
			_ = queue.Decide(queue.Pending()[0].ID, "ana", false)
			err := guarita(queue).Authorize(t.Context(), deleteDB)
			again := errors.Is(err, approval.ErrRecentlyDenied)
			t.Logf("%-11s o operador vê %2d pedidos; a repetição "+
				"negada na hora? %v", c.name, seen, again)
			if (c.name == "com limites") != (seen == 3 && again) {
				t.Errorf("%s: seen %d, again %v", c.name, seen, again)
			}
		})
	}
}

// #endregion

// insist faz o agente pedir n vezes, cada pedido numa goroutine, e
// devolve quantos chegaram ao operador.
func insist(t *testing.T, queue *approval.Queue, n int) int {
	uc := guarita(queue)
	for range n {
		go func() { _ = uc.Authorize(t.Context(), deleteDB) }()
	}
	synctest.Wait()
	return len(queue.Pending())
}
