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
// PolicyEvaluator decide o que fazer com uma chamada já validada, a
// partir de quem pede, em que servidor e o quê.
type PolicyEvaluator interface {
	Evaluate(
		ctx context.Context, input domain.PolicyInput,
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

// #region app-intercept-requester
// WithServer diz à política em que servidor a chamada vai rodar: o
// namespace da rota, conhecido quando a Guarita é montada.
func WithServer(server string) Option {
	return func(uc *InterceptCall) { uc.server = server }
}

// WithAgent diz como saber quem é o agente de um pedido. O caso de uso
// não conhece tokens: recebe uma função do contexto para o nome.
func WithAgent(agent func(context.Context) string) Option {
	return func(uc *InterceptCall) { uc.agent = agent }
}

// #endregion

var _ ToolCatalog = (*PinCatalog)(nil)

// admissible valida a chamada e, só se ela for válida, consulta o
// catálogo.
func (uc *InterceptCall) admissible(
	ctx context.Context, call domain.Call,
) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return uc.checkCatalog(ctx, call)
}

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
	policy           PolicyEvaluator
	upstream         Upstream
	catalog          ToolCatalog
	server           string
	agent            func(context.Context) string
	approver         Approver
	inspectors       []Inspector
	findings         FindingsObserver
	resultInspectors []ResultInspector
	marker           ResultMarker
	auditor          Auditor
}

// #endregion

// #region app-intercept-new
// NewInterceptCall recebe as portas de que o caso de uso precisa. As
// opcionais chegam por opções funcionais.
func NewInterceptCall(
	policy PolicyEvaluator, upstream Upstream, options ...Option,
) *InterceptCall {
	uc := &InterceptCall{
		policy: policy, upstream: upstream,
		agent:    func(context.Context) string { return "" },
		findings: func(domain.PolicyInput, domain.Inspection) {},
	}
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
// Authorize decide a chamada sem repassá-la, e registra a decisão na
// trilha de auditoria, se houver uma: nil só para allow.
func (uc *InterceptCall) Authorize(
	ctx context.Context, call domain.Call,
) error {
	decision, err := uc.decide(ctx, call)
	return uc.audit(ctx, call, decision, err)
}

// #endregion

// #region app-intercept-decide
// decide é a decisão de sempre: validação, catálogo, política,
// inspetores e, se for o caso, a aprovação. Devolve também o que a
// política e os inspetores decidiram, para o registro.
func (uc *InterceptCall) decide(
	ctx context.Context, call domain.Call,
) (domain.Decision, error) {
	deny := domain.DecisionDeny
	if err := uc.admissible(ctx, call); err != nil {
		return deny, fmt.Errorf("intercept call: %w", err)
	}
	decision, err := uc.evaluate(ctx, call)
	if err != nil {
		return deny, fmt.Errorf("intercept call: %w: %w",
			ErrPolicyFailed, err)
	}
	if decision, err = uc.inspect(ctx, call, decision); err != nil {
		return deny, fmt.Errorf("intercept call: %w", err)
	}
	switch decision {
	case domain.DecisionAllow:
		return decision, nil
	case domain.DecisionRequireApproval:
		return decision, uc.approve(ctx, call)
	case domain.DecisionDeny:
		return decision, ErrPolicyDenied
	}
	return deny, ErrPolicyFailed
}

// #endregion

// evaluate monta a entrada da política e confere a decisão que volta.
func (uc *InterceptCall) evaluate(
	ctx context.Context, call domain.Call,
) (domain.Decision, error) {
	decision, err := uc.policy.Evaluate(ctx, uc.input(ctx, call))
	if err == nil {
		err = decision.Validate()
	}
	return decision, err
}

// input é o que a política e o aprovador recebem sobre a chamada.
func (uc *InterceptCall) input(
	ctx context.Context, call domain.Call,
) domain.PolicyInput {
	return domain.PolicyInput{
		Agent: uc.agent(ctx), Server: uc.server, Call: call,
	}
}

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
