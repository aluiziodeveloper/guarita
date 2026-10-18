package app

import (
	"context"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region app-approver
// Approver leva a chamada a uma pessoa e espera a decisão. Devolve o
// estado final do pedido; um erro quer dizer que o pedido nem entrou na
// fila, e a chamada é negada.
type Approver interface {
	Await(
		ctx context.Context, input domain.PolicyInput,
	) (domain.ApprovalState, error)
}

// WithApprover faz require_approval esperar uma pessoa. Sem ele, a
// chamada que pede aprovação é negada, como no Capítulo 11.
func WithApprover(approver Approver) Option {
	return func(uc *InterceptCall) { uc.approver = approver }
}

// #endregion

// #region app-intercept-approve
// approve só devolve nil para um pedido aprovado. Recusado, expirado,
// cancelado ou fora da fila, a chamada é negada com o motivo.
func (uc *InterceptCall) approve(
	ctx context.Context, call domain.Call,
) error {
	if uc.approver == nil {
		return ErrApprovalRequired
	}
	state, err := uc.approver.Await(ctx, uc.input(ctx, call))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrApprovalRequired, err)
	}
	if state != domain.ApprovalApproved {
		return fmt.Errorf("%w: %s", ErrApprovalRequired, state)
	}
	return nil
}

// #endregion
