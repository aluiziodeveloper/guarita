package dlp

import "regexp"

// cpfCandidate casa com onze dígitos, com ou sem pontos e hífen. \b
// impede que onze dígitos de um número maior casem: entre dois
// dígitos não há fronteira de palavra.
var cpfCandidate = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)

// #region dlp-valid-cpf
// ValidCPF confere os dois dígitos verificadores. Cada um é a soma dos
// dígitos anteriores com pesos decrescentes, vezes 10, módulo 11; o
// resto 10 vale 0. Onze dígitos iguais têm verificadores certos e não
// são CPF.
func ValidCPF(candidate string) bool {
	digits := make([]int, 0, 11)
	for _, r := range candidate {
		if '0' <= r && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) != 11 || allSame(digits) {
		return false
	}
	for _, size := range []int{9, 10} {
		sum := 0
		for i, d := range digits[:size] {
			sum += d * (size + 1 - i)
		}
		if sum*10%11%10 != digits[size] {
			return false
		}
	}
	return true
}

// #endregion

func allSame(digits []int) bool {
	for _, d := range digits[1:] {
		if d != digits[0] {
			return false
		}
	}
	return true
}
