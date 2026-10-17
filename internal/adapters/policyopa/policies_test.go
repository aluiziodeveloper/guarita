package policyopa

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// policyCase é uma chamada e a decisão que a política deve dar.
type policyCase struct {
	agent, server, tool, arguments string
	want                           domain.Decision
}

// policyCases cobre concessão, ausência de concessão, aprovação,
// ferramenta concedida em outro servidor, argumentos e agente anônimo.
var policyCases = []policyCase{
	{"relatorios", "files", "read_file", `{"path":"q3.txt"}`, "allow"},
	{"relatorios", "db", "delete_database", `{}`, "deny"},
	{"relatorios", "db", "purge_records", `{}`, "deny"},
	{"manutencao", "db", "purge_records", `{}`, "require_approval"},
	{"manutencao", "files", "delete_database", `{}`, "deny"},
	{"relatorios", "files", "read_file", `{"path":"/etc/x"}`, "deny"},
	{"relatorios", "files", "read_file", `{"path":"a/../../x"}`, "deny"},
	{"", "files", "read_file", `{"path":"q3.txt"}`, "deny"},
}

// #region policyopa-policies-test
// A política versionada em policies/, a mesma que a Guarita carrega na
// partida, contra a tabela de casos.
func TestPolicies_menorPrivilegio(t *testing.T) {
	policy, err := Load(t.Context(), os.DirFS("../../../policies"))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	for _, c := range policyCases {
		raw := json.RawMessage(c.arguments)
		got, err := policy.Evaluate(t.Context(), domain.PolicyInput{
			Agent: c.agent, Server: c.server,
			Call: domain.Call{ToolName: c.tool, Arguments: raw},
		})
		t.Logf("%-10s %-5s %-15s %-20s -> %s", c.agent, c.server,
			c.tool, c.arguments, got)
		if err != nil || got != c.want {
			t.Errorf("%+v: %s, %v", c, got, err)
		}
	}
}

// #endregion
