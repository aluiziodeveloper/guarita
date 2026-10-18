// Package artifact confere um artefato antes de a Guarita confiar nele:
// o executável de um servidor stdio, pelo SHA-256 do arquivo, ou a
// imagem da sandbox, pelo digest na referência.
package artifact

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Erros da conferência.
var (
	ErrMismatch = errors.New("artifact: digest does not match the approved")
	ErrUnpinned = errors.New("artifact: image is not pinned by digest")
)

// #region artifact-resolve
// Resolve acha o executável como o sistema o acharia, pelo PATH, e
// devolve o caminho real, sem links simbólicos. A Guarita confere e
// executa esse caminho, e não o nome: um diretório novo no PATH, ou
// um link trocado, não muda o arquivo que foi conferido.
func Resolve(name string) (string, error) {
	found, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("artifact: %w", err)
	}
	abs, err := filepath.Abs(found)
	if err != nil {
		return "", fmt.Errorf("artifact: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("artifact: %w", err)
	}
	return resolved, nil
}

// #endregion

// #region artifact-check
// Check calcula o digest do arquivo e o compara com o aprovado. O erro
// traz os dois digests, que não são segredo, para quem decide se a
// mudança foi uma atualização esperada.
func Check(path string, approved domain.Digest) error {
	//nolint:gosec // G304: o caminho vem do operador
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	defer func() { _ = file.Close() }()
	got, err := domain.DigestOf(file)
	if err != nil {
		return err
	}
	if got != approved {
		return fmt.Errorf("%w: %s is %s, approved %s", ErrMismatch,
			filepath.Base(path), got, approved)
	}
	return nil
}

// #endregion

// #region artifact-image
// CheckImage exige que a imagem seja referida pelo digest, como em
// nome@sha256:…, e que ele seja o aprovado. Com o digest na
// referência, quem confere os bytes da imagem é o próprio Docker, ao
// baixá-la e ao executá-la; uma etiqueta como :1.0 pode mudar de
// conteúdo, e um digest, não.
func CheckImage(ref string, approved domain.Digest) error {
	_, pinned, ok := strings.Cut(ref, "@")
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnpinned, ref)
	}
	if domain.Digest(pinned) != approved {
		return fmt.Errorf("%w: %s, approved %s", ErrMismatch, ref,
			approved)
	}
	return nil
}

// #endregion
