package domain

import (
	"errors"
	"strings"
	"testing"
)

// #region domain-digest-test
// O digest do texto vazio é o da documentação do SHA-256; os formatos
// errados são recusados antes de virar aprovação.
func TestDigest(t *testing.T) {
	const empty = "sha256:e3b0c44298fc1c149afbf4c8996fb924" +
		"27ae41e4649b934ca495991b7852b855"
	got, err := DigestOf(strings.NewReader(""))
	if err != nil || got != empty {
		t.Fatalf("DigestOf(\"\") = %s, %v", got, err)
	}
	for _, bad := range []string{
		"", empty[7:], empty[:20],
		"sha256:" + strings.ToUpper(empty[7:]),
		"sha512:" + empty[7:], empty + "0",
	} {
		_, err := ParseDigest(bad)
		if !errors.Is(err, ErrInvalidDigest) {
			t.Errorf("ParseDigest(%.24q): %v", bad, err)
		}
	}
	if d, err := ParseDigest(empty); err != nil || d != got {
		t.Errorf("ParseDigest(válido) = %s, %v", d, err)
	}
}

// #endregion
