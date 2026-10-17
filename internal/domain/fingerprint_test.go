package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func definition(raw string) ToolDefinition {
	return ToolDefinition{Name: "read_file", Raw: []byte(raw)}
}

// load lê uma definição de testdata/tools.
func load(t *testing.T, name string) ToolDefinition {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "tools", name))
	if err != nil {
		t.Fatalf("ler %s: %v", name, err)
	}
	return ToolDefinition{Name: "read_file", Raw: raw}
}

// #region domain-fingerprint-test
// Forma diferente, mesmo conteúdo: mesma impressão digital. Conteúdo
// diferente, na descrição ou no esquema: outra.
func TestFingerprint_cobreADefinicaoInteira(t *testing.T) {
	base, _ := load(t, "base.json").Fingerprint()
	for _, c := range []struct {
		file string
		same bool
	}{
		{"reordered.json", true},
		{"escaped.json", true},
		{"description.json", false},
		{"schema.json", false},
	} {
		file, same := c.file, c.same
		got, err := load(t, file).Fingerprint()
		t.Logf("%-16s %s", file, got)
		if err != nil || (got == base) != same {
			t.Errorf("%s: %s, base %s, err=%v", file, got, base, err)
		}
	}
}

// #endregion

func TestFingerprint_ehOSHA256DaFormaCanonica(t *testing.T) {
	got, _ := definition(`{ "name" : "read_file" }`).Fingerprint()
	sum := sha256.Sum256([]byte(`{"name":"read_file"}`))
	if want := "sha256:" + hex.EncodeToString(sum[:]); string(got) != want {
		t.Fatalf("Fingerprint() = %s; want %s", got, want)
	}
}

func TestFingerprint_recusaOQueNaoEhObjeto(t *testing.T) {
	for _, raw := range []string{`[]`, `"x"`, `{"name":1}{}`, `{`} {
		if _, err := definition(raw).Fingerprint(); !errors.Is(err,
			ErrInvalidDefinition) {
			t.Errorf("%s: err = %v", raw, err)
		}
	}
	if _, err := (ToolDefinition{Raw: []byte(`{}`)}).Fingerprint(); !errors.
		Is(err, ErrInvalidDefinition) {
		t.Errorf("sem nome: err = %v", err)
	}
}
