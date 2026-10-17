package policystatic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

func TestPolicy_negaFerramentaAusente(t *testing.T) {
	policy, err := New(map[string]domain.Decision{
		"read_file": domain.DecisionAllow,
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	for tool, want := range map[string]domain.Decision{
		"read_file":   domain.DecisionAllow,
		"delete_file": domain.DecisionDeny,
		"":            domain.DecisionDeny,
	} {
		got, err := policy.Evaluate(context.Background(),
			domain.PolicyInput{Call: domain.Call{ToolName: tool}})
		if err != nil || got != want {
			t.Errorf("Evaluate(%q) = %s, %v; want %s", tool, got, err, want)
		}
	}
}

func TestNew_recusaDecisaoInvalida(t *testing.T) {
	_, err := New(map[string]domain.Decision{"read_file": "execute"})
	if !errors.Is(err, domain.ErrInvalidDecision) {
		t.Fatalf("New() = %v; want ErrInvalidDecision", err)
	}
}

func TestNew_copiaATabela(t *testing.T) {
	decisions := map[string]domain.Decision{"read_file": domain.DecisionAllow}
	policy, err := New(decisions)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	decisions["delete_file"] = domain.DecisionAllow

	got, _ := policy.Evaluate(context.Background(),
		domain.PolicyInput{Call: domain.Call{ToolName: "delete_file"}})
	if got != domain.DecisionDeny {
		t.Fatalf("Evaluate(delete_file) = %s; a tabela vazou", got)
	}
}

type recordingUpstream struct{ tools []string }

func (u *recordingUpstream) Forward(
	_ context.Context, call domain.Call,
) (domain.Result, error) {
	u.tools = append(u.tools, call.ToolName)
	return domain.Result{}, nil
}

// #region policystatic-integration-test
// O adaptador real encaixa na porta: o caso de uso nega o que a tabela
// não permite, sem rede e sem dublê de política.
func TestInterceptCall_comPoliticaEstatica(t *testing.T) {
	policy, err := New(map[string]domain.Decision{
		"read_file": domain.DecisionAllow,
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	upstream := &recordingUpstream{}
	uc := app.NewInterceptCall(policy, upstream)
	args := json.RawMessage(`{}`)

	for _, tool := range []string{"read_file", "delete_file"} {
		_, _ = uc.Run(context.Background(),
			domain.Call{ToolName: tool, Arguments: args})
	}

	if len(upstream.tools) != 1 || upstream.tools[0] != "read_file" {
		t.Fatalf("repassadas = %v; want [read_file]", upstream.tools)
	}
}

// #endregion
