package policyopa

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do Capítulo 11: relatorios pode chamar send_email, no
// servidor mail, sem aprovação só para destinatários de lab.test;
// para qualquer outro, pede aprovação; sem destinatário, nega.
var challengeCases = []policyCase{
	{"relatorios", "mail", "send_email", `{"to":"ana@lab.test"}`, "allow"},
	{"relatorios", "mail", "send_email", `{"to":"ANA@LAB.TEST"}`, "allow"},
	{
		"relatorios", "mail", "send_email", `{"to":"x@example.com"}`,
		"require_approval",
	},
	{
		"relatorios", "mail", "send_email", `{"to":"a@lab.test.example"}`,
		"require_approval",
	},
	{"relatorios", "mail", "send_email", `{}`, "deny"},
	{"relatorios", "mail", "send_email", `{"to":42}`, "deny"},
	{"manutencao", "mail", "send_email", `{"to":"ana@lab.test"}`, "deny"},
}

func TestPolicies_desafioSendEmail(t *testing.T) {
	policy, err := Load(t.Context(), os.DirFS("../../../policies"))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	for _, c := range challengeCases {
		raw := json.RawMessage(c.arguments)
		got, err := policy.Evaluate(t.Context(), domain.PolicyInput{
			Agent: c.agent, Server: c.server,
			Call: domain.Call{ToolName: c.tool, Arguments: raw},
		})
		if err != nil || got != c.want {
			t.Errorf("%s %s: %s, %v; want %s",
				c.agent, c.arguments, got, err, c.want)
		}
	}
}
