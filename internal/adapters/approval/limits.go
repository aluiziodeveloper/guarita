package approval

import (
	"errors"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var (
	// ErrTooManyPending recusa o agente que já tem pedidos demais na
	// fila: insistir não aumenta a chance de alguém aprovar.
	ErrTooManyPending = errors.New("too many pending approvals")
	// ErrRecentlyDenied recusa a mesma chamada logo depois de negada.
	ErrRecentlyDenied = errors.New("same call was denied recently")
)

// Option ajusta a fila.
type Option func(*Queue)

// #region approval-limits
// WithMaxPending limita os pedidos pendentes de cada agente.
func WithMaxPending(n int) Option {
	return func(q *Queue) { q.maxPending = n }
}

// WithCooldown nega na hora, por um tempo, a mesma chamada que um
// operador acabou de negar: mesmo agente, servidor, ferramenta e
// argumentos, na forma canônica do Capítulo 9.
func WithCooldown(d time.Duration) Option {
	return func(q *Queue) { q.cooldown = d }
}

// #endregion

// #region approval-admit
// admit decide, sob o mutex, se o pedido pode entrar na fila.
func (q *Queue) admit(input domain.PolicyInput, key string) error {
	if until, ok := q.denied[key]; ok && time.Now().Before(until) {
		return ErrRecentlyDenied
	}
	pending := 0
	for _, e := range q.entries {
		if e.request.State == domain.ApprovalPending &&
			e.request.Input.Agent == input.Agent {
			pending++
		}
	}
	if q.maxPending > 0 && pending >= q.maxPending {
		return ErrTooManyPending
	}
	return nil
}

// #endregion

// callKey identifica a chamada para a repetição. Argumentos que não
// têm forma canônica ficam como vieram.
func callKey(input domain.PolicyInput) string {
	arguments, err := domain.Canonical(input.Call.Arguments)
	if err != nil {
		arguments = input.Call.Arguments
	}
	return input.Agent + "\x00" + input.Server + "\x00" +
		input.Call.ToolName + "\x00" + string(arguments)
}
