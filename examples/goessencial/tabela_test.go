package goessencial

import (
	"errors"
	"testing"
)

// #region testes-tabela
// Um teste orientado a tabela: cada caso é uma linha, e t.Run dá a ele
// um nome, que aparece na saída e pode ser escolhido com -run.
func TestDecide(t *testing.T) {
	for _, c := range []struct {
		name, tool string
		want       error
	}{
		{"leitura permitida", "read_file", nil},
		{"apagar o banco", "delete_database", ErrDenied},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := decide(c.tool); !errors.Is(err, c.want) {
				t.Errorf("decide(%q) = %v, quer %v", c.tool, err, c.want)
			}
		})
	}
}

// #endregion
