// Package policystatic é o adaptador de política mais simples: uma
// tabela fixa de decisões por nome de ferramenta, que nega o resto.
package policystatic

import (
	"context"
	"fmt"
	"maps"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// O compilador confere que Policy satisfaz a porta do caso de uso.
var _ app.PolicyEvaluator = (*Policy)(nil)

// #region policystatic-policy
// Policy decide pelo nome da ferramenta. Ferramenta ausente é negada.
type Policy struct {
	decisions map[string]domain.Decision
}

// New copia a tabela e recusa decisões fora do conjunto do domínio.
func New(decisions map[string]domain.Decision) (*Policy, error) {
	for tool, decision := range decisions {
		if err := decision.Validate(); err != nil {
			return nil, fmt.Errorf("policy for %q: %w", tool, err)
		}
	}
	return &Policy{decisions: maps.Clone(decisions)}, nil
}

// Evaluate devolve a decisão da tabela ou DecisionDeny.
func (p *Policy) Evaluate(
	_ context.Context, input domain.PolicyInput,
) (domain.Decision, error) {
	if decision, ok := p.decisions[input.Call.ToolName]; ok {
		return decision, nil
	}
	return domain.DecisionDeny, nil
}

// #endregion
