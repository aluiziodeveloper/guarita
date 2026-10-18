package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrInvalidDigest recusa um digest que não é "sha256:" e 64 dígitos
// hexadecimais minúsculos.
var ErrInvalidDigest = errors.New(
	"digest must be sha256: and 64 lowercase hex digits")

// #region domain-digest
// Digest identifica um artefato pelo conteúdo: "sha256:" e o resumo em
// hex, o mesmo formato dos digests de imagens de contêiner. Dois
// artefatos com o mesmo digest têm os mesmos bytes, qualquer que seja
// o nome ou a versão que declarem.
type Digest string

// ParseDigest confere o formato antes de o digest virar uma aprovação:
// um digest mal copiado nunca confere com nada, e a falha seria
// confundida com um artefato trocado.
func ParseDigest(s string) (Digest, error) {
	hexPart, ok := strings.CutPrefix(s, "sha256:")
	if !ok || len(hexPart) != 2*sha256.Size ||
		strings.ToLower(hexPart) != hexPart {
		return "", ErrInvalidDigest
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return "", ErrInvalidDigest
	}
	return Digest(s), nil
}

// #endregion

// #region domain-digest-of
// DigestOf lê r até o fim e devolve o digest do que leu. O resumo é
// calculado por partes, enquanto os bytes passam, e um executável de
// centenas de megabytes não precisa caber na memória.
func DigestOf(r io.Reader) (Digest, error) {
	h := sha256.New() // um hash.Hash também é um io.Writer
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("digest: %w", err)
	}
	return Digest("sha256:" + hex.EncodeToString(h.Sum(nil))), nil
}

// #endregion
