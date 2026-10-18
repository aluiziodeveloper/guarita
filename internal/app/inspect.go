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

// #region app-run-stages
// runStages passa um sujeito pelos inspetores, na ordem dada, e serve
// aos dois sentidos: I é Inspector na ida e ResultInspector na volta.
// A decisão mais severa vence; a fila para no primeiro inspetor que
// stop aponta, e ele volta como o motivo, ou nil, se ninguém parou.
// Um erro para tudo.
func runStages[I any](
	stages []I,
	inspect func(I) (domain.Inspection, error),
	observe func(domain.Inspection),
	stop func(domain.Inspection) bool,
) (domain.Decision, *domain.Inspection, error) {
	decision := domain.DecisionAllow
	for _, s := range stages {
		inspection, err := inspect(s)
		if err != nil {
			return domain.DecisionDeny, nil,
				fmt.Errorf("%w: %w", ErrInspectionFailed, err)
		}
		if len(inspection.Findings) > 0 {
			observe(inspection)
		}
		decision = domain.Stricter(decision, inspection.Decision)
		if stop(inspection) {
			return decision, &inspection, nil
		}
	}
	return decision, nil, nil
}

// #endregion

// #region app-intercept-inspect
// inspect passa pelos inspetores a chamada que a política não negou. A
// decisão mais severa vence; negar devolve o motivo, sem os dados.
func (uc *InterceptCall) inspect(
	ctx context.Context, call domain.Call, decision domain.Decision,
) (domain.Decision, error) {
	if decision == domain.DecisionDeny {
		return decision, nil
	}
	input := uc.input(ctx, call)
	found, stopper, err := runStages(uc.inspectors,
		func(i Inspector) (domain.Inspection, error) {
			return i.Inspect(ctx, input)
		},
		func(in domain.Inspection) { uc.findings(input, in) },
		func(in domain.Inspection) bool {
			return in.Decision == domain.DecisionDeny
		})
	if err != nil {
		return domain.DecisionDeny, err
	}
	if stopper != nil {
		return found, fmt.Errorf("%w: %s", ErrInspectionDenied,
			summary(stopper.Findings))
	}
	return domain.Stricter(decision, found), nil
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
