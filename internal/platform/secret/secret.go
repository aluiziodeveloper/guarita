// Package secret guarda credenciais num tipo que não se deixa imprimir:
// o valor só sai por Reveal, chamado onde ele é de fato usado.
package secret

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// redacted é o que aparece no lugar do segredo em texto, JSON e logs.
const redacted = "[REDACTED]"

// Erros da leitura de um segredo em arquivo.
var (
	ErrEmpty       = errors.New("secret is empty")
	ErrPermissions = errors.New("secret file is readable by others")
)

// #region secret-type
// Secret é uma credencial. String, GoString, LogValue e MarshalJSON
// escondem o valor: um log ou um %v descuidado não o vaza.
type Secret struct {
	value string
}

// New embrulha um valor.
func New(value string) Secret { return Secret{value: value} }

// Reveal devolve o valor. Cada chamada é um ponto a revisar.
func (s Secret) Reveal() string { return s.value }

// IsZero informa se não há segredo configurado.
func (s Secret) IsZero() bool { return s.value == "" }

func (Secret) String() string   { return redacted }
func (Secret) GoString() string { return redacted }

// LogValue faz slog registrar o marcador no lugar do valor.
func (Secret) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON faz o mesmo em JSON.
func (Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redacted + `"`), nil
}

// #endregion

// #region secret-read-file
// ReadFile lê um segredo de arquivo, sem espaços nas pontas. O arquivo
// não pode ser legível pelo grupo nem pelos demais usuários. Permissão
// e conteúdo vêm do mesmo descritor, sem intervalo entre os dois.
func ReadFile(path string) (Secret, error) {
	//nolint:gosec // G304: o caminho vem da configuração do operador
	file, err := os.Open(path)
	if err != nil {
		return Secret{}, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return Secret{}, err
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return Secret{}, fmt.Errorf("%w: %s has mode %v",
			ErrPermissions, path, mode)
	}
	data, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err != nil {
		return Secret{}, err
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return Secret{}, ErrEmpty
	}
	return New(value), nil
}

// #endregion
