package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var (
	// ErrInspectionDenied indica que um inspetor negou a chamada.
	ErrInspectionDenied = errors.New("call denied by inspection")
	// ErrInspectionFailed indica que um inspetor não conseguiu
	// examinar a chamada; ela é negada.
	ErrInspectionFailed = errors.New("inspection failed")
)

// #region app-inspector
// Inspector examina o conteúdo de uma chamada que a política não negou.
// Um erro quer dizer que ele não conseguiu examinar, e a chamada é
// negada.
type Inspector interface {
	Inspect(
		ctx context.Context, input domain.PolicyInput,
	) (domain.Inspection, error)
}

// WithInspectors acrescenta inspetores, consultados na ordem dada.
func WithInspectors(inspectors ...Inspector) Option {
	return func(uc *InterceptCall) {
		uc.inspectors = append(uc.inspectors, inspectors...)
	}
}

// FindingsObserver recebe a chamada e a inspeção que teve achados.
type FindingsObserver func(domain.PolicyInput, domain.Inspection)

// WithFindings recebe cada inspeção com achados, para o registro.
func WithFindings(observe FindingsObserver) Option {
	return func(uc *InterceptCall) { uc.findings = observe }
}

// #endregion

// #region app-intercept-inspect
// inspect passa pelos inspetores a chamada que a política não negou. A
// decisão mais severa vence; negar devolve o motivo, sem os dados.
func (uc *InterceptCall) inspect(
	ctx context.Context, call domain.Call, decision domain.Decision,
) (domain.Decision, error) {
	input := uc.input(ctx, call)
	for _, inspector := range uc.inspectors {
		if decision == domain.DecisionDeny {
			break
		}
		inspection, err := inspector.Inspect(ctx, input)
		if err != nil {
			return domain.DecisionDeny,
				fmt.Errorf("%w: %w", ErrInspectionFailed, err)
		}
		if len(inspection.Findings) > 0 {
			uc.findings(input, inspection)
		}
		decision = domain.Stricter(decision, inspection.Decision)
		if inspection.Decision == domain.DecisionDeny {
			return decision, fmt.Errorf("%w: %s", ErrInspectionDenied,
				summary(inspection.Findings))
		}
	}
	return decision, nil
}

// #endregion

// summary descreve os achados pelo sinal e pelo campo, como "cpf em
// body"; o detalhe fica para o registro do operador.
func summary(findings []domain.Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, f.Signal+" em "+f.Field)
	}
	return strings.Join(parts, "; ")
}
