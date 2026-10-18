// Package naivedlp mostra o DLP ingênuo do Capítulo 13: uma expressão
// regular para o CPF formatado e uma lista de nomes de campo que
// parecem guardar segredos.
package naivedlp

import (
	"encoding/json"
	"regexp"
	"strings"
)

// #region examples-naivedlp
// formattedCPF casa com o CPF escrito com pontos e hífen.
var formattedCPF = regexp.MustCompile(`\d{3}\.\d{3}\.\d{3}-\d{2}`)

// secretFields são nomes de campo que costumam guardar segredos.
var secretFields = []string{"password", "secret", "token", "api_key"}

// Leaks olha os campos de primeiro nível: o nome e, se for texto, o
// valor.
func Leaks(arguments json.RawMessage) bool {
	var fields map[string]any
	_ = json.Unmarshal(arguments, &fields)
	for name, value := range fields {
		for _, word := range secretFields {
			if strings.Contains(strings.ToLower(name), word) {
				return true
			}
		}
		text, ok := value.(string)
		if ok && formattedCPF.MatchString(text) {
			return true
		}
	}
	return false
}

// #endregion
