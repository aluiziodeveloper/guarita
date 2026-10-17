package scanner

import (
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do Capítulo 10: seletores de variação e o combining grapheme
// joiner são marcas combinantes (Mn), como os acentos, e passam pelo
// varredor. O sinal esperado é "invisible"; os acentos continuam limpos.
func TestScan_desafioMarcasInvisiveis(t *testing.T) {
	cases := []struct {
		name, text string
		want       bool // espera o sinal invisible
	}{
		{"seletor FE0F", "Le um arquivo.\uFE0F[LAB]", true},
		{"seletor FE00", "Le\uFE00 um arquivo.", true},
		{"seletor suplementar", "Le um arquivo.\U000E0100", true},
		{"grapheme joiner", "Le um a\u034Frquivo.", true},
		{"agudo decomposto", "Le\u0301 um arquivo.", false},
		{"cedilha decomposta", "ac\u0327a\u0303o", false},
		{"til composto", "ação e função", false},
	}
	for _, c := range cases {
		raw := `{"name":"read_file","description":"` + c.text + `"}`
		found := Structural{}.Scan(domain.ToolDefinition{
			Name: "read_file", Raw: []byte(raw),
		})
		got := false
		for _, f := range found {
			got = got || f.Signal == "invisible"
		}
		if got != c.want {
			t.Errorf("%s: invisible = %v, quer %v (%+v)",
				c.name, got, c.want, found)
		}
	}
}
