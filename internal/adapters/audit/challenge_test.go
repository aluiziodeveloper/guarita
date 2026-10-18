package audit

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do Capítulo 19: a cabeça guardada à parte, mas assinada. Sign
// assina com Ed25519 o número e o hash de uma entrada, num Checkpoint.
// VerifyAgainst lê a trilha e confere a cadeia e o checkpoint: a
// assinatura precisa valer, e a entrada com aquele número precisa ter
// aquele hash. Entradas depois do checkpoint são aceitas. O pacote não
// compila até existirem Checkpoint, Sign, VerifyAgainst, ErrCheckpoint
// e ErrSignature.
func TestCheckpoint_desafio(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "a.jsonl")
	trail, _ := Open(path, nil)
	var cp Checkpoint
	for n := 1; n <= 5; n++ {
		_ = trail.Record(t.Context(), entry("web_search", "allow"), nil)
		if n == 3 {
			cp = Sign(priv, trail.Head())
		}
	}
	_ = trail.Close()
	raw, _ := os.ReadFile(path)
	lines := strings.SplitAfter(string(raw), "\n")
	for _, c := range []struct {
		name, trail string
		key         ed25519.PublicKey
		want        error
	}{
		{"intacta, com entradas depois", string(raw), pub, nil},
		{
			"fim cortado antes do checkpoint", strings.Join(lines[:2], ""),
			pub, ErrCheckpoint,
		},
		{"cadeia refeita do meio", rewrite(lines), pub, ErrCheckpoint},
		{"checkpoint de outra chave", string(raw), other, ErrSignature},
	} {
		err := VerifyAgainst(bytes.NewBufferString(c.trail), c.key, cp)
		t.Logf("%-32s %v", c.name, err)
		if (c.want == nil) != (err == nil) || !errors.Is(err, c.want) {
			t.Errorf("%s: %v; quer %v", c.name, err, c.want)
		}
	}
}

// rewrite troca a decisão da segunda entrada e refaz a cadeia dali em
// diante, como faria quem conhece o algoritmo.
func rewrite(lines []string) string {
	var out strings.Builder
	var prev domain.AuditEntry
	for n, line := range lines {
		if line == "" {
			continue
		}
		var e domain.AuditEntry
		_ = json.Unmarshal([]byte(line), &e)
		if n == 1 {
			e.Decision = domain.DecisionDeny
		}
		if n >= 1 {
			e.Prev = prev.Hash
			e = e.Seal()
		}
		b, _ := json.Marshal(e)
		out.Write(append(b, '\n'))
		prev = e
	}
	return out.String()
}
