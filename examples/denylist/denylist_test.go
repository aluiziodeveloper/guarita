package denylist

import "testing"

// #region examples-denylist-test
// A lista global foi feita para o agente de manutenção e vale para o de
// relatórios. A negação pega delete_database e deixa purge_records.
func TestPolicy_listaGlobalENegacao(t *testing.T) {
	tools := []string{"read_file", "delete_database", "purge_records"}
	policy := Policy{Allow: tools, Deny: []string{"delete_*", "drop_*"}}
	for _, tool := range tools {
		t.Logf("relatorios pede %-15s permitido? %v",
			tool, policy.Allowed(tool))
	}
	if policy.Allowed("delete_database") ||
		!policy.Allowed("purge_records") {
		t.Fatal("a lista de negação deveria pegar só o nome previsto")
	}
}

// #endregion
