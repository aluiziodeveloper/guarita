package app

import (
	"context"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// fixedInspector devolve uma inspeção pronta e conta as consultas.
type fixedInspector struct {
	inspection domain.Inspection
	err        error
	calls      int
}

func (i *fixedInspector) Inspect(
	context.Context, domain.PolicyInput,
) (domain.Inspection, error) {
	i.calls++
	return i.inspection, i.err
}

func finds(decision domain.Decision, signal string) *fixedInspector {
	return &fixedInspector{inspection: domain.Inspection{
		Decision: decision,
		Findings: []domain.Finding{{Field: "body", Signal: signal}},
	}}
}

// clean é o inspetor que não acha nada.
func clean() *fixedInspector {
	return &fixedInspector{
		inspection: domain.Inspection{Decision: domain.DecisionAllow},
	}
}

// #region app-inspect-test
// A política permite; os inspetores decidem por cima dela, e o mais
// severo vence. Um inspetor que falha nega.
func TestAuthorize_inspetoresDepoisDaPolitica(t *testing.T) {
	allow := &stubPolicy{decision: domain.DecisionAllow}
	approve, deny := domain.DecisionRequireApproval, domain.DecisionDeny
	for _, c := range []struct {
		name      string
		inspector *fixedInspector
		want      error
	}{
		{"nada achado", clean(), nil},
		{"cpf: aprovação", finds(approve, "cpf"), ErrApprovalRequired},
		{"chave: nega", finds(deny, "secret"), ErrInspectionDenied},
		{
			"inspetor falha", &fixedInspector{err: errors.New("x")},
			ErrInspectionFailed,
		},
	} {
		uc := NewInterceptCall(allow, &spyUpstream{},
			WithInspectors(c.inspector))
		err := uc.Authorize(context.Background(), validCall)
		t.Logf("%-18s -> %v", c.name, err)
		if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: %v; want %v", c.name, err, c.want)
		}
	}
}

// #endregion

func TestAuthorize_politicaQueNegaNaoChamaInspetor(t *testing.T) {
	inspector := finds(domain.DecisionAllow, "")
	uc := NewInterceptCall(&stubPolicy{decision: domain.DecisionDeny},
		&spyUpstream{}, WithInspectors(inspector))
	err := uc.Authorize(context.Background(), validCall)
	if !errors.Is(err, ErrPolicyDenied) || inspector.calls != 0 {
		t.Fatalf("%v; inspetor chamado %d vezes", err, inspector.calls)
	}
}
