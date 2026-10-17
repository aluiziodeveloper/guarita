package scanner

import (
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

func def(name, raw string) domain.ToolDefinition {
	return domain.ToolDefinition{Name: name, Raw: []byte(raw)}
}

func signals(findings []domain.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Field+"="+f.Signal+"("+f.Detail+")")
	}
	return out
}

// #region scanner-test
// Cada caso traz um marcador [LAB] disfarçado de um jeito; os dois
// primeiros são texto legítimo e não podem gerar sinal.
func TestScan_sinais(t *testing.T) {
	for _, c := range []struct {
		name, raw, want string
	}{
		{"limpa", `{"description":"Lê um arquivo."}`, ""},
		{"acento decomposto", `{"description":"Lê um arquivo."}`, ""},
		{"largura zero", `{"description":"Lê.\u200b[LAB]"}`, "format"},
		{"direção", `{"inputSchema":{"properties":{"path":` +
			`{"description":"\u202e[LAB]"}}}}`, "format"},
		{"tags Unicode", `{"description":"Lê.\udb40\udc4c"}`, "format"},
		{"controle", `{"description":"Lê.\u0007"}`, "control"},
		{
			"largura total", `{"description":"ＬＡＢ"}`,
			"compatibility",
		},
		{"chave", `{"descr\u200biption":"Lê."}`, "format"},
	} {
		got := signals(Structural{}.Scan(def("read_file", c.raw)))
		t.Logf("%-17s -> %v", c.name, got)
		if (c.want == "") != (len(got) == 0) ||
			(c.want != "" && !contains(got, c.want)) {
			t.Errorf("%s: %v; want %q", c.name, got, c.want)
		}
	}
}

// #endregion

func contains(got []string, signal string) bool {
	for _, entry := range got {
		if strings.Contains(entry, "="+signal+"(") {
			return true
		}
	}
	return false
}

func TestScan_nome(t *testing.T) {
	for name, valid := range map[string]bool{
		"read_file": true, "admin.tools.list": true, "DATA_EXPORT_v2": true,
		"read file": false, "read\u200bfile": false, "": false,
		"ler_arquivo_ç": false,
	} {
		got := Structural{}.Scan(def(name, `{}`))
		if (len(got) == 0) != valid {
			t.Errorf("%q: %v", name, signals(got))
		}
	}
}
