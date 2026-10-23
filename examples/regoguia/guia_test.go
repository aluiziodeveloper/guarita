package regoguia

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/tester"
)

// modules lê os módulos de rego/, menos os testes, como a Guarita faz
// com policies/.
func modules() []func(*rego.Rego) {
	paths, err := filepath.Glob("rego/*.rego")
	if err != nil {
		log.Fatal(err)
	}
	var options []func(*rego.Rego)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.rego") {
			continue
		}
		source, err := os.ReadFile(path) //nolint:gosec // G304: rego/ do exemplo
		if err != nil {
			log.Fatal(err)
		}
		options = append(options, rego.Module(path, string(source)))
	}
	return options
}

// eval avalia a consulta com a entrada, com os erros estritos de
// funções embutidas ligados, e escreve o valor em JSON, "indefinido"
// quando nenhuma regra vale, ou o erro.
func eval(query string, input any) string {
	options := append(modules(), rego.Query(query),
		rego.StrictBuiltinErrors(true))
	if input != nil {
		options = append(options, rego.Input(input))
	}
	results, err := rego.New(options...).Eval(context.Background())
	if err != nil {
		return "erro: " + err.Error()
	}
	if len(results) == 0 {
		return "indefinido"
	}
	value, _ := json.Marshal(results[0].Expressions[0].Value)
	return string(value)
}

// #region regoguia-regras
// Uma regra completa dá um valor quando o corpo vale; o default
// responde quando nenhuma vale; sem default, a regra fica indefinida.
func Example_regrasCompletas() {
	for _, in := range []map[string]any{
		{},
		{"tool": "read_file", "server": "files"},
		{"tool": "delete_database", "agent": "manutencao"},
		{"tool": "delete_database", "agent": "relatorios"},
	} {
		fmt.Println(eval("data.guia.regras.decision", in),
			eval("data.guia.regras.owner", in))
	}
	// Output:
	// "deny" indefinido
	// "allow" "files"
	// "require_approval" indefinido
	// "deny" indefinido
}

// #endregion

// #region regoguia-conjuntos
// Conjuntos, objetos, every e compreensões, com duas entradas.
func Example_conjuntos() {
	fmt.Println(eval("data.guia.conjuntos.reasons",
		map[string]any{"path": "/etc/../passwd"}))
	fmt.Println(eval("data.guia.conjuntos.reasons",
		map[string]any{"path": "docs/relatorio.txt"}))
	fmt.Println(eval("data.guia.conjuntos.tools_per_server", nil))
	fmt.Println(eval("data.guia.conjuntos.all_tools", nil))
	fmt.Println(eval("data.guia.conjuntos.all_known",
		map[string]any{"tools": []string{"read_file", "list_dir"}}),
		eval("data.guia.conjuntos.all_known",
			map[string]any{"tools": []string{"read_file", "send_email"}}))
	// Output:
	// ["caminho absoluto","caminho sai do diretório"]
	// []
	// {"db":1,"files":2}
	// ["delete_database","list_dir","read_file"]
	// true indefinido
}

// #endregion

// #region regoguia-funcoes
// Uma função própria, valores padrão com object.get e o erro estrito
// de uma função embutida que recebe o tipo errado.
func Example_funcoes() {
	for _, to := range []any{"Ana@Lab.Test", "ana@lab.test.example", nil, 42} {
		args := map[string]any{}
		if to != nil {
			args["to"] = to
		}
		fmt.Println(eval("data.guia.funcoes.message",
			map[string]any{"arguments": args}))
	}
	// Output:
	// "para \"Ana@Lab.Test\": allow"
	// "para \"ana@lab.test.example\": require_approval"
	// "para \"\": require_approval"
	// erro: rego/funcoes.rego:5: eval_type_error: lower: operand 1 must be string but got number
}

// #endregion

// #region regoguia-armadilhas
// O not vale quando a expressão é indefinida, e não só quando ela é
// falsa; duas regras completas com valores diferentes são um erro.
func Example_armadilhas() {
	for _, blocked := range []any{true, "true", nil} {
		in := map[string]any{}
		if blocked != nil {
			in["blocked"] = blocked
		}
		shown, _ := json.Marshal(blocked)
		if blocked == nil {
			shown = []byte("ausente")
		}
		fmt.Printf("blocked %-7s → %s\n", shown,
			eval("data.guia.armadilhas.decision", in))
	}
	fmt.Println(eval("data.guia.armadilhas.conflict",
		map[string]any{"tool": "list_dir"}))
	fmt.Println(eval("data.guia.armadilhas.conflict",
		map[string]any{"tool": "read_file"}))
	// Output:
	// blocked true    → indefinido
	// blocked "true"  → "allow"
	// blocked ausente → "allow"
	// indefinido
	// erro: rego/armadilhas.rego:12: eval_conflict_error: complete rules must not produce multiple outputs
}

// #endregion

// #region regoguia-opa-test
// Os testes em Rego de rego/ rodam também no go test, pelo mesmo
// executor do opa test, e cada teste que falha vira uma falha aqui.
func TestRegoTests(t *testing.T) {
	mods, store, err := tester.Load([]string{"rego"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	results, err := tester.NewRunner().SetStore(store).SetModules(mods).
		RunTests(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for r := range results {
		if r.Fail || r.Error != nil {
			t.Errorf("%s: falhou: %v", r.Name, r.Error)
		}
		names = append(names, r.Name)
	}
	slices.Sort(names)
	t.Logf("%d testes em Rego: %v", len(names), names)
	if len(names) != 5 {
		t.Errorf("quer 5 testes, rodaram %d", len(names))
	}
}

// #endregion
