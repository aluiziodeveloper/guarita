// Package naiveinjection mostra a defesa ingênua do Capítulo 14:
// procurar, no resultado de uma ferramenta, as frases conhecidas de
// injeção de prompt.
package naiveinjection

import "strings"

// #region examples-naiveinjection
// phrases são as frases que todo mundo já viu num ataque.
var phrases = []string{
	"ignore as instruções anteriores",
	"ignore previous instructions",
}

// Blocked diz se o resultado tem uma das frases, sem diferenciar
// maiúsculas.
func Blocked(result string) bool {
	lower := strings.ToLower(result)
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// #endregion
