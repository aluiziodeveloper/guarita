package dlp

import (
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// withLuhn completa os dígitos com o verificador de Luhn: dobra um
// dígito sim, outro não, a partir da direita, e soma os algarismos.
func withLuhn(base string) string {
	sum := 0
	for i := range len(base) {
		d := int(base[len(base)-1-i] - '0')
		if i%2 == 0 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return base + string(rune('0'+(10-sum%10)%10))
}

// Desafio do Capítulo 13: um detector de cartão, com o sinal "card".
// Um número de 13 a 19 dígitos, com espaços ou hífens entre grupos, que
// passa no Luhn é cartão; o que não passa, ou é feito de um dígito só,
// não é. A tabela padrão não tem "card", e um sinal sem decisão nega.
func TestInspect_desafioCartao(t *testing.T) {
	card := withLuhn("411111111111111") // 16 dígitos
	grouped := card[:4] + " " + card[4:8] + " " + card[8:12] + " " +
		card[12:]
	wrong := card[:15] + string(rune('0'+(card[15]-'0'+1)%10))
	for _, c := range []struct {
		name, text string
		card       bool
	}{
		{"cartão junto", "pagamento " + card, true},
		{"cartão em grupos", "cartão " + grouped, true},
		{"cartão com hífens", strings.ReplaceAll(grouped, " ", "-"), true},
		{"Luhn errado", "pagamento " + wrong, false},
		{"dígito só", "código 0000000000000000", false},
		{"pedido de 12 dígitos", "pedido 123456789012", false},
	} {
		got := inspectText(c.text)
		found := false
		for _, f := range got.Findings {
			found = found || f.Signal == "card"
		}
		if found != c.card ||
			(c.card && got.Decision != domain.DecisionDeny) {
			t.Errorf("%s: card=%v, %s", c.name, found, got.Decision)
		}
	}
}
