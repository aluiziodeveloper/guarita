package approval

import (
	"errors"
	"slices"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ErrAlreadySigned recusa a segunda assinatura do mesmo operador.
var ErrAlreadySigned = errors.New("operator already signed this request")

// WithQuorum exige n operadores diferentes para aprovar a ferramenta.
func WithQuorum(tool string, n int) Option {
	return func(q *Queue) {
		if q.quorum == nil {
			q.quorum = map[string]int{}
		}
		q.quorum[tool] = n
	}
}

// sign registra a assinatura de quem aprova e diz se o quórum fechou.
// Um pedido que já saiu de pending segue para finish, que recusa.
func (q *Queue) sign(e *entry, operator string) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	need := q.quorum[e.request.Input.Call.ToolName]
	if e.request.State != domain.ApprovalPending || need <= 1 {
		return true, nil
	}
	if slices.Contains(e.request.Signers, operator) {
		return false, ErrAlreadySigned
	}
	e.request.Signers = append(e.request.Signers, operator)
	return len(e.request.Signers) >= need, nil
}
