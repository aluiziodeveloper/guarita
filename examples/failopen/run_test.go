package failopen

import (
	"context"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

type fixedPolicy struct {
	decision domain.Decision
	err      error
}

func (p fixedPolicy) Evaluate(
	context.Context, domain.PolicyInput,
) (domain.Decision, error) {
	return p.decision, p.err
}

type countingUpstream struct{ calls int }

func (u *countingUpstream) Forward(
	context.Context, domain.Call,
) (domain.Result, error) {
	u.calls++
	return domain.Result{}, nil
}

// #region examples-failopen-test
// Os dois casos fixam a lacuna: o que não é deny chega ao servidor.
func TestRun_repassaODesconhecido(t *testing.T) {
	for _, policy := range []fixedPolicy{
		{decision: "execute"},
		{err: errors.New("policy store unavailable")},
	} {
		upstream := &countingUpstream{}
		_, err := Run(context.Background(), policy, upstream,
			domain.Call{ToolName: "read_file"})
		if err != nil || upstream.calls != 1 {
			t.Fatalf("Run() = %v, repasses %d; a versão ingênua "+
				"repassa", err, upstream.calls)
		}
	}
}

// #endregion
