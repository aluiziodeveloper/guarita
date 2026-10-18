// Package naivefiles mostra a leitura ingênua de arquivos do Capítulo
// 17: o caminho pedido é limpo, juntado à raiz e conferido pelo
// prefixo, e o sistema operacional segue os symlinks que encontrar.
package naivefiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutside indica um caminho que sai da raiz.
var ErrOutside = errors.New("path outside root")

// #region examples-naivefiles
// Read lê name dentro de root. filepath.Join limpa o caminho, e o
// prefixo recusa o "..": "../segredo" vira um caminho fora da raiz.
// O que a conferência não vê é o que o caminho aponta: um symlink
// dentro da raiz, para fora dela, passa no prefixo, e os.ReadFile o
// segue.
func Read(root, name string) ([]byte, error) {
	path := filepath.Join(root, name)
	prefix := filepath.Clean(root) + string(os.PathSeparator)
	if !strings.HasPrefix(path, prefix) {
		return nil, ErrOutside
	}
	//nolint:gosec // G304 é o ponto: a defesa ingênua do capítulo
	return os.ReadFile(path)
}

// #endregion
