package policyopa

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"testing/fstest"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region policyopa-test
// os.DirFS dá o diretório como fs.FS; o _test.rego, inválido de
// propósito, fica de fora.
func TestPolicy_decide(t *testing.T) {
	ctx := t.Context() // cancelado quando o teste termina
	policy, err := Load(ctx, os.DirFS("testdata/minimal"))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	allow, deny := domain.DecisionAllow, domain.DecisionDeny
	approve := domain.DecisionRequireApproval
	for _, c := range []struct {
		tool, arguments string
		want            domain.Decision
	}{
		{"read_file", `{}`, allow},
		{"write_file", `{"path":"plan.txt"}`, approve},
		{"write_file", `{"path":"notes.txt"}`, deny},
		{"delete_database", `{}`, deny},
	} {
		raw := json.RawMessage(c.arguments)
		got, err := policy.Evaluate(ctx, domain.PolicyInput{
			Call: domain.Call{ToolName: c.tool, Arguments: raw},
		})
		t.Logf("%-15s %-22s -> %s", c.tool, c.arguments, got)
		if err != nil || got != c.want {
			t.Errorf("%s: %s, %v; want %s", c.tool, got, err, c.want)
		}
	}
}

// #endregion

// Sem valor padrão e sem regra que case, a decisão é indefinida.
func TestPolicy_indefinidaEhErro(t *testing.T) {
	ctx := context.Background()
	policy, err := Load(ctx, fstest.MapFS{"p.rego": {Data: []byte(
		"package guarita\n\ndecision := \"allow\" if input.tool == \"x\"\n",
	)}})
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	_, err = policy.Evaluate(ctx, domain.PolicyInput{
		Call: domain.Call{ToolName: "y"},
	})
	if !errors.Is(err, ErrUndefined) {
		t.Fatalf("Evaluate() = %v; want ErrUndefined", err)
	}
}

// Regra com dois valores ao mesmo tempo é erro de avaliação, não uma
// das duas decisões.
func TestPolicy_conflitoEhErro(t *testing.T) {
	ctx := context.Background()
	policy, err := Load(ctx, fstest.MapFS{"p.rego": {Data: []byte(
		"package guarita\n\ndecision := \"allow\" if true\n\n" +
			"decision := \"deny\" if true\n",
	)}})
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	_, err = policy.Evaluate(ctx, domain.PolicyInput{})
	t.Logf("conflito: %v", err)
	if err == nil {
		t.Fatal("o conflito deveria ser erro")
	}
}

func TestLoad_recusaPoliticaInvalidaOuVazia(t *testing.T) {
	ctx := context.Background()
	_, err := Load(ctx, fstest.MapFS{"p.rego": {Data: []byte(
		"package guarita\n\ndecision := \n",
	)}})
	t.Logf("sintaxe: %v", err)
	if err == nil {
		t.Fatal("Load() deveria recusar a sintaxe")
	}
	if _, err := Load(ctx, fstest.MapFS{}); !errors.Is(err,
		ErrNoModules) {
		t.Fatalf("Load(vazio) = %v", err)
	}
}
