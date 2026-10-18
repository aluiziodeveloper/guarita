package domain

import (
	"errors"
	"testing"
)

// #region domain-approval-test
// Todas as combinações de estado de origem e de destino: só as quatro
// saídas de pending são aceitas.
func TestApprovalState_next(t *testing.T) {
	states := []ApprovalState{
		ApprovalPending, ApprovalApproved, ApprovalDenied,
		ApprovalExpired, ApprovalCanceled,
	}
	accepted := 0
	for _, from := range states {
		for _, to := range states {
			got, err := from.Next(to)
			ok := from == ApprovalPending && to != ApprovalPending
			if ok != (err == nil) || (!ok && got != from) ||
				(!ok && !errors.Is(err, ErrInvalidTransition)) {
				t.Errorf("%s -> %s = %s, %v", from, to, got, err)
			}
			if err == nil {
				accepted++
			}
		}
	}
	t.Logf("%d de %d transições aceitas; terminais: %v",
		accepted, len(states)*len(states), terminals(states))
}

// #endregion

func terminals(states []ApprovalState) []ApprovalState {
	var out []ApprovalState
	for _, s := range states {
		if s.Terminal() {
			out = append(out, s)
		}
	}
	return out
}
