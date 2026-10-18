package app

import (
	"context"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// stubApprover devolve um estado pronto e guarda a entrada.
type stubApprover struct {
	state domain.ApprovalState
	err   error
	input domain.PolicyInput
}

func (a *stubApprover) Await(
	_ context.Context, input domain.PolicyInput,
) (domain.ApprovalState, error) {
	a.input = input
	return a.state, a.err
}

// ends devolve um aprovador que termina no estado dado.
func ends(state domain.ApprovalState) Approver {
	return &stubApprover{state: state}
}

// #region app-approval-test
// Só approved deixa a chamada seguir; os outros estados, um erro do
// aprovador ou a falta dele negam com ErrApprovalRequired.
func TestAuthorize_esperaAAprovacao(t *testing.T) {
	policy := &stubPolicy{decision: domain.DecisionRequireApproval}
	for _, c := range []struct {
		name     string
		approver Approver
		allowed  bool
	}{
		{"sem aprovador", nil, false},
		{"approved", ends(domain.ApprovalApproved), true},
		{"denied", ends(domain.ApprovalDenied), false},
		{"expired", ends(domain.ApprovalExpired), false},
		{"fila cheia", &stubApprover{err: errors.New("full")}, false},
	} {
		var options []Option
		if c.approver != nil {
			options = append(options, WithApprover(c.approver))
		}
		uc := NewInterceptCall(policy, &spyUpstream{}, options...)
		err := uc.Authorize(context.Background(), validCall)
		t.Logf("%-13s -> %v", c.name, err)
		if (err == nil) != c.allowed ||
			(err != nil && !errors.Is(err, ErrApprovalRequired)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// #endregion
