package domain

import (
	"errors"
	"fmt"
)

// ApprovalState é o estado de um pedido de aprovação humana.
type ApprovalState string

// #region domain-approval-states
// Os estados de um pedido. Só pending tem saída; os outros quatro são
// terminais: decidido uma vez, o pedido não muda mais.
const (
	ApprovalPending  ApprovalState = "pending"  // espera uma pessoa
	ApprovalApproved ApprovalState = "approved" // a chamada segue
	ApprovalDenied   ApprovalState = "denied"   // uma pessoa recusou
	ApprovalExpired  ApprovalState = "expired"  // o prazo acabou
	ApprovalCanceled ApprovalState = "canceled" // o agente desistiu
)

// transitions é a tabela da máquina de estados: de cada estado, para
// onde ele pode ir. Um estado ausente da tabela é terminal.
var transitions = map[ApprovalState][]ApprovalState{
	ApprovalPending: {
		ApprovalApproved, ApprovalDenied, ApprovalExpired,
		ApprovalCanceled,
	},
}

// #endregion

// ErrInvalidTransition recusa uma mudança de estado fora da tabela.
var ErrInvalidTransition = errors.New("invalid approval transition")

// #region domain-approval-next
// Next devolve to se a tabela permite ir de s para to. Aprovar duas
// vezes, ou aprovar o que expirou, é erro, e não um caso a ignorar.
func (s ApprovalState) Next(to ApprovalState) (ApprovalState, error) {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return to, nil
		}
	}
	return s, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s, to)
}

// Terminal diz se o pedido já foi decidido.
func (s ApprovalState) Terminal() bool {
	return len(transitions[s]) == 0
}

// #endregion
