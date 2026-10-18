package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// fixedResult devolve uma inspeção pronta para qualquer resultado.
type fixedResult struct{ inspection domain.Inspection }

func (f fixedResult) InspectResult(
	context.Context, domain.PolicyInput, json.RawMessage,
) (domain.Inspection, error) {
	return f.inspection, nil
}

// prefix marca o resultado pondo o servidor na frente.
type prefix struct{}

func (prefix) Mark(
	input domain.PolicyInput, result json.RawMessage,
) (json.RawMessage, error) {
	return json.RawMessage(`{"from":"` + input.Server + `","result":` +
		string(result) + `}`), nil
}

// #region app-review-result-test
// O resultado limpo passa marcado; o que um inspetor não permite é
// barrado inteiro, sem chegar ao marcador.
func TestReviewResult(t *testing.T) {
	result := json.RawMessage(`{"content":[{"text":"ok"}]}`)
	allow := domain.Inspection{Decision: domain.DecisionAllow}
	clean := fixedResult{allow}
	hostile := fixedResult{domain.Inspection{
		Decision: domain.DecisionDeny,
		Findings: []domain.Finding{{
			Field:  "content[0].text",
			Signal: "role-marker",
		}},
	}}
	for _, c := range []struct {
		name      string
		inspector ResultInspector
	}{{"limpo", clean}, {"com marca de papel", hostile}} {
		uc := NewInterceptCall(&stubPolicy{}, &spyUpstream{},
			WithServer("web"), WithResultInspectors(c.inspector),
			WithResultMarker(prefix{}))
		got, err := uc.ReviewResult(t.Context(), validCall, result)
		t.Logf("%-18s -> %s %v", c.name, got, err)
		if (err == nil) != (c.name == "limpo") ||
			(err != nil && !errors.Is(err, ErrResultBlocked)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// #endregion

// #region app-review-empty-decision-test
// Um inspetor que devolve uma decisão vazia, ou fora do conjunto, não
// permitiu: o resultado é barrado. A inspeção zero, que o laço devolvia
// quando ninguém parava, tem essa mesma decisão vazia, e por isso quem
// parou volta como ponteiro, e não como valor.
func TestReviewResult_decisaoVazia(t *testing.T) {
	result := json.RawMessage(`{"content":[{"text":"ok"}]}`)
	for _, decision := range []domain.Decision{"", "talvez"} {
		uc := NewInterceptCall(nil, nil, WithResultInspectors(
			fixedResult{domain.Inspection{Decision: decision}}))
		_, err := uc.ReviewResult(t.Context(),
			domain.Call{ToolName: "web_search"}, result)
		t.Logf("decisão %q: %v", decision, err)
		if !errors.Is(err, ErrResultBlocked) {
			t.Errorf("decisão %q passou", decision)
		}
	}
}

// #endregion
