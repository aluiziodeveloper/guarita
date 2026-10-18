package admin

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

// ErrOperators recusa um arquivo de operadores malformado.
var ErrOperators = errors.New("operators: each line is 'name secret'")

// Operators guarda o segredo de cada operador. É outra credencial, e
// não o token do agente: quem pede não é quem aprova.
type Operators map[string]secret.Secret

// ParseOperators lê uma linha "nome segredo" por operador. Linhas em
// branco e começadas por # ficam de fora.
func ParseOperators(text string) (Operators, error) {
	operators := Operators{}
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, " ")
		value = strings.TrimSpace(value)
		if !ok || name == "" || len(value) < 16 {
			return nil, fmt.Errorf("%w (%q)", ErrOperators, name)
		}
		operators[name] = secret.New(value)
	}
	if len(operators) == 0 {
		return nil, ErrOperators
	}
	return operators, nil
}

// #region admin-authenticate
// Authenticate devolve o operador do pedido. Aceita Bearer, da linha de
// comando, e Basic, do navegador. A comparação leva o mesmo tempo para
// segredos certos e errados, e todos os operadores são comparados.
func (ops Operators) Authenticate(r *http.Request) (string, bool) {
	user, given, basic := r.BasicAuth()
	if !basic {
		bearer, found := strings.CutPrefix(
			r.Header.Get("Authorization"), "Bearer ")
		if !found {
			return "", false
		}
		given = bearer
	}
	operator := ""
	for name, want := range ops {
		match := subtle.ConstantTimeCompare([]byte(given),
			[]byte(want.Reveal())) == 1
		if match && (!basic || user == name) {
			operator = name
		}
	}
	return operator, operator != ""
}

// #endregion
