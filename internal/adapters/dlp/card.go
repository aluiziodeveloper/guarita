package dlp

import "regexp"

// SignalCard é um número de cartão de pagamento.
const SignalCard = "card"

// cardCandidate casa com 13 a 19 dígitos, com um espaço ou hífen
// opcional entre eles, como os cartões são escritos.
var cardCandidate = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)

// ValidCard aplica o algoritmo de Luhn: a partir da direita, dobra um
// dígito sim, outro não, subtrai 9 do que passar de 9, e a soma de
// tudo precisa ser múltipla de 10. Um dígito só repetido não conta.
func ValidCard(candidate string) bool {
	var digits []int
	for _, r := range candidate {
		if '0' <= r && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 || allSame(digits) {
		return false
	}
	sum := 0
	for i := range digits {
		d := digits[len(digits)-1-i]
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}
