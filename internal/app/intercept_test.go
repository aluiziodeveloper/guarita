package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region app-test-doubles
// stubPolicy devolve uma decisão pronta e conta as consultas.
type stubPolicy struct {
	decision domain.Decision
	err      error
	calls    int
}

func (p *stubPolicy) Evaluate(
	context.Context, domain.PolicyInput,
) (domain.Decision, error) {
	p.calls++
	return p.decision, p.err
}

// spyUpstream registra cada chamada repassada.
type spyUpstream struct {
	forwarded []domain.Call
	err       error
}

func (u *spyUpstream) Forward(
	_ context.Context, call domain.Call,
) (domain.Result, error) {
	u.forwarded = append(u.forwarded, call)
	return domain.Result{Content: json.RawMessage(`{"ok":true}`)}, u.err
}

// #endregion

var validCall = domain.Call{
	ToolName:  "read_file",
	Arguments: json.RawMessage(`{"path":"notes.txt"}`),
}

func TestInterceptCall_repassaSoComAllow(t *testing.T) {
	// #region app-test-table
	errBoom := errors.New("policy store unavailable")
	tests := []struct {
		name        string
		decision    domain.Decision
		policyErr   error
		wantErr     error
		wantForward int
	}{
		{"allow repassa", domain.DecisionAllow, nil, nil, 1},
		{"deny nega", domain.DecisionDeny, nil, ErrPolicyDenied, 0},
		{
			"aprovação pausa", domain.DecisionRequireApproval, nil,
			ErrApprovalRequired, 0,
		},
		{"desconhecida nega", "execute", nil, ErrPolicyFailed, 0},
		{"decisão vazia nega", "", nil, ErrPolicyFailed, 0},
		{"erro da política nega", "", errBoom, ErrPolicyFailed, 0},
	}
	// #endregion
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := &stubPolicy{decision: test.decision, err: test.policyErr}
			upstream := &spyUpstream{}
			uc := NewInterceptCall(policy, upstream)

			_, err := uc.Run(context.Background(), validCall)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Run() error = %v; want %v", err, test.wantErr)
			}
			if got := len(upstream.forwarded); got != test.wantForward {
				t.Fatalf("repasses = %d; want %d", got, test.wantForward)
			}
			if test.policyErr != nil && !errors.Is(err, test.policyErr) {
				t.Fatalf("Run() error = %v; perdeu a causa", err)
			}
		})
	}
}

func TestInterceptCall_validaAntesDaPolitica(t *testing.T) {
	policy := &stubPolicy{decision: domain.DecisionAllow}
	upstream := &spyUpstream{}
	uc := NewInterceptCall(policy, upstream)
	call := domain.Call{ToolName: "read_file", Arguments: []byte(`{"path":`)}

	_, err := uc.Run(context.Background(), call)

	if !errors.Is(err, domain.ErrInvalidArguments) {
		t.Fatalf("Run() error = %v; want ErrInvalidArguments", err)
	}
	if policy.calls != 0 || len(upstream.forwarded) != 0 {
		t.Fatalf("política %d, repasses %d; want 0 e 0",
			policy.calls, len(upstream.forwarded))
	}
}

func TestInterceptCall_devolveResultadoEErroDoServidor(t *testing.T) {
	policy := &stubPolicy{decision: domain.DecisionAllow}
	upstream := &spyUpstream{}
	uc := NewInterceptCall(policy, upstream)

	result, err := uc.Run(context.Background(), validCall)
	if err != nil || string(result.Content) != `{"ok":true}` {
		t.Fatalf("Run() = %s, %v; want o resultado do servidor",
			result.Content, err)
	}

	upstream.err = errors.New("connection refused")
	if _, err := uc.Run(context.Background(), validCall); !errors.Is(
		err, upstream.err) {
		t.Fatalf("Run() error = %v; want a causa do servidor", err)
	}
}

type ctxKey struct{}

// contextPolicy confere que o contexto de quem chamou chegou à porta.
type contextPolicy struct{ seen any }

func (p *contextPolicy) Evaluate(
	ctx context.Context, _ domain.PolicyInput,
) (domain.Decision, error) {
	p.seen = ctx.Value(ctxKey{})
	return domain.DecisionDeny, nil
}

func TestInterceptCall_propagaOContexto(t *testing.T) {
	policy := &contextPolicy{}
	uc := NewInterceptCall(policy, &spyUpstream{})
	ctx := context.WithValue(context.Background(), ctxKey{}, "req-42")

	_, _ = uc.Run(ctx, validCall)

	if policy.seen != "req-42" {
		t.Fatalf("contexto na política = %v; want req-42", policy.seen)
	}
}

func TestInterceptCall_autorizaSemRepassar(t *testing.T) {
	upstream := &spyUpstream{}
	for decision, want := range map[domain.Decision]error{
		domain.DecisionAllow:           nil,
		domain.DecisionDeny:            ErrPolicyDenied,
		domain.DecisionRequireApproval: ErrApprovalRequired,
		"execute":                      ErrPolicyFailed,
	} {
		uc := NewInterceptCall(&stubPolicy{decision: decision}, upstream)
		err := uc.Authorize(context.Background(), validCall)
		if !errors.Is(err, want) {
			t.Errorf("Authorize(%s) = %v; want %v", decision, err, want)
		}
	}
	if len(upstream.forwarded) != 0 {
		t.Fatalf("repasses = %d; Authorize não repassa",
			len(upstream.forwarded))
	}
}

// inputPolicy guarda a entrada que recebeu.
type inputPolicy struct{ input domain.PolicyInput }

func (p *inputPolicy) Evaluate(
	_ context.Context, input domain.PolicyInput,
) (domain.Decision, error) {
	p.input = input
	return domain.DecisionDeny, nil
}

// #region app-intercept-input-test
// A política recebe o agente tirado do contexto, o servidor da rota e a
// chamada, e nada mais.
func TestAuthorize_montaAEntradaDaPolitica(t *testing.T) {
	policy := &inputPolicy{}
	agent := func(ctx context.Context) string {
		name, _ := ctx.Value(ctxKey{}).(string)
		return name
	}
	uc := NewInterceptCall(policy, &spyUpstream{},
		WithServer("files"), WithAgent(agent))
	ctx := context.WithValue(context.Background(), ctxKey{},
		"relatorios")
	_ = uc.Authorize(ctx, validCall)
	want := domain.PolicyInput{
		Agent: "relatorios", Server: "files", Call: validCall,
	}
	if policy.input.Agent != want.Agent ||
		policy.input.Server != want.Server ||
		policy.input.Call.ToolName != want.Call.ToolName {
		t.Fatalf("entrada = %+v; want %+v", policy.input, want)
	}
}

// #endregion
