// Package app reúne os casos de uso da Guarita e as portas que eles
// consomem. Ele depende só do domínio; os adaptadores dependem dele.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region app-ports
// PolicyEvaluator decide o que fazer com uma chamada já validada.
type PolicyEvaluator interface {
	Evaluate(
		ctx context.Context, call domain.Call,
	) (domain.Decision, error)
}

// Upstream repassa uma chamada permitida ao servidor MCP real.
type Upstream interface {
	Forward(
		ctx context.Context, call domain.Call,
	) (domain.Result, error)
}

// #endregion

var (
	// ErrPolicyDenied indica que a política negou a chamada.
	ErrPolicyDenied = errors.New("call denied by policy")
	// ErrApprovalRequired indica que a chamada espera aprovação humana.
	ErrApprovalRequired = errors.New("call requires human approval")
	// ErrPolicyFailed indica que a política não produziu uma decisão
	// válida; a chamada é negada.
	ErrPolicyFailed = errors.New("policy could not decide")
)

// #region app-tool-catalog
// ToolCatalog diz se uma ferramenta pode ser chamada no estado atual do
// catálogo. *PinCatalog o satisfaz.
type ToolCatalog interface {
	Check(ctx context.Context, tool string) error
}

// Option ajusta o caso de uso.
type Option func(*InterceptCall)

// WithCatalog faz Authorize negar a ferramenta fora do catálogo ou com
// definição alterada, antes de consultar a política.
func WithCatalog(catalog ToolCatalog) Option {
	return func(uc *InterceptCall) { uc.catalog = catalog }
}

// #endregion

var _ ToolCatalog = (*PinCatalog)(nil)

// checkCatalog consulta o catálogo, se houver um.
func (uc *InterceptCall) checkCatalog(
	ctx context.Context, call domain.Call,
) error {
	if uc.catalog == nil {
		return nil
	}
	return uc.catalog.Check(ctx, call.ToolName)
}

// #region app-intercept-type
// InterceptCall é o caso de uso que fica entre o agente e o servidor.
type InterceptCall struct {
	policy   PolicyEvaluator
	upstream Upstream
	catalog  ToolCatalog
}

// NewInterceptCall recebe as portas de que o caso de uso precisa. As
// opcionais chegam por opções funcionais.
func NewInterceptCall(
	policy PolicyEvaluator, upstream Upstream, options ...Option,
) *InterceptCall {
	uc := &InterceptCall{policy: policy, upstream: upstream}
	for _, option := range options {
		option(uc)
	}
	return uc
}

// #endregion

// #region app-intercept-run
// Run autoriza e, se a decisão for DecisionAllow, repassa ao servidor.
func (uc *InterceptCall) Run(
	ctx context.Context, call domain.Call,
) (domain.Result, error) {
	if err := uc.Authorize(ctx, call); err != nil {
		return domain.Result{}, err
	}
	return uc.forward(ctx, call)
}

// #endregion

// #region app-intercept-authorize
// Authorize valida a chamada e pergunta à política, sem repassar nada.
// Devolve nil só para DecisionAllow; qualquer outro caso nega.
func (uc *InterceptCall) Authorize(
	ctx context.Context, call domain.Call,
) error {
	if err := call.Validate(); err != nil {
		return fmt.Errorf("intercept call: %w", err)
	}
	if err := uc.checkCatalog(ctx, call); err != nil {
		return fmt.Errorf("intercept call: %w", err)
	}
	decision, err := uc.policy.Evaluate(ctx, call)
	if err == nil {
		err = decision.Validate()
	}
	if err != nil {
		return fmt.Errorf("intercept call: %w: %w",
			ErrPolicyFailed, err)
	}
	switch decision {
	case domain.DecisionAllow:
		return nil
	case domain.DecisionRequireApproval:
		return ErrApprovalRequired
	case domain.DecisionDeny:
		return ErrPolicyDenied
	}
	return ErrPolicyFailed
}

// #endregion

func (uc *InterceptCall) forward(
	ctx context.Context, call domain.Call,
) (domain.Result, error) {
	result, err := uc.upstream.Forward(ctx, call)
	if err != nil {
		return domain.Result{}, fmt.Errorf("intercept call: forward: %w",
			err)
	}
	return result, nil
}
