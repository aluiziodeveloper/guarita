package artifact

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region artifact-stage
// Stage copia o executável para dir enquanto calcula o digest. O que é
// conferido é o que foi copiado, byte a byte, e a cópia fica num
// diretório só da Guarita: trocar o original depois da conferência não
// muda o que vai rodar. Com outro digest, a cópia é apagada.
func Stage(
	path string, approved domain.Digest, dir string,
) (string, error) {
	if err := private(dir); err != nil {
		return "", err
	}
	//nolint:gosec // G304: o caminho vem do operador
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("artifact: %w", err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.CreateTemp(dir, filepath.Base(path)+"-*") // 0o600
	if err != nil {
		return "", fmt.Errorf("artifact: %w", err)
	}
	got, err := domain.DigestOf(io.TeeReader(src, dst))
	return keep(dst, got, approved, errors.Join(err, dst.Close()))
}

// #endregion

// #region artifact-keep
// keep deixa a cópia executável só pelo dono, se ela for a aprovada, ou
// a apaga.
func keep(
	dst *os.File, got, approved domain.Digest, err error,
) (string, error) {
	if err == nil && got != approved {
		err = fmt.Errorf("%w: copy is %s, approved %s", ErrMismatch,
			got, approved)
	}
	if err != nil {
		_ = os.Remove(dst.Name())
		return "", err
	}
	//nolint:gosec // G302: a cópia precisa ser executável pelo dono
	return dst.Name(), os.Chmod(dst.Name(), 0o500)
}

// #endregion

// private cria dir, se preciso, e o deixa só para o dono: MkdirAll
// respeita a umask e não muda um diretório que já existe.
func private(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	//nolint:gosec // G302: um diretório precisa de x para ser aberto
	return os.Chmod(dir, 0o700)
}
