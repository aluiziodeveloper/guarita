// Package naivescan mostra as duas defesas ingênuas do Capítulo 10:
// procurar palavras na descrição e reescrevê-la com NFKC.
package naivescan

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// #region examples-naivescan
// HasBlockedWord procura, sem diferenciar maiúsculas, cada palavra da
// lista na descrição.
func HasBlockedWord(description string, words []string) bool {
	lower := strings.ToLower(description)
	for _, word := range words {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// RewriteNFKC devolve a descrição na forma de compatibilidade, para que
// o agente receba um texto "limpo".
func RewriteNFKC(description string) string {
	return norm.NFKC.String(description)
}

// #endregion
