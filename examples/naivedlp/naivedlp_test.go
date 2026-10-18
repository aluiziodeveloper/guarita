package naivedlp

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// cpf completa nove dígitos com os dois verificadores, para que o
// teste não grave nenhum CPF pronto no repositório.
func cpf(base string) string {
	digits := []byte(base)
	for _, size := range []int{9, 10} {
		sum := 0
		for i := range size {
			sum += int(digits[i]-'0') * (size + 1 - i)
		}
		check := sum * 10 % 11 % 10
		digits = append(digits, byte('0'+check))
	}
	return string(digits)
}

// #region examples-naivedlp-test
// Três vazamentos que passam e um número de pedido que não é CPF, mas
// tem o formato dele.
func TestLeaks_contornosEFalsoPositivo(t *testing.T) {
	plain := cpf("123456789") // sem pontuação
	key := "AKIA" + "LABLABLABLABLAB0"
	encoded := base64.StdEncoding.EncodeToString([]byte(plain))
	for _, c := range []struct {
		name, arguments string
		caught          bool
	}{
		{"CPF sem pontuação", `{"body":"` + plain + `"}`, false},
		{"chave no campo note", `{"note":"` + key + `"}`, false},
		{"CPF em base64", `{"body":"` + encoded + `"}`, false},
		{"pedido 123.456.789-00", `{"order":"123.456.789-00"}`, true},
	} {
		got := Leaks(json.RawMessage(c.arguments))
		t.Logf("%-22s acusado? %v", c.name, got)
		if got != c.caught {
			t.Errorf("%s: o ingênuo mudou de comportamento", c.name)
		}
	}
}

// #endregion
