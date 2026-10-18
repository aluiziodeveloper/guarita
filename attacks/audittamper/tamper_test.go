package audittamper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/audit"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// record grava cinco decisões e devolve as linhas e a cabeça, o hash
// que o operador guardou à parte.
func record(t *testing.T) ([]string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auditoria.jsonl")
	trail, err := audit.Open(path, []byte("chave-de-laboratorio"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []domain.Decision{
		"allow", "deny", "allow",
		"deny", "allow",
	} {
		_ = trail.Record(t.Context(), domain.AuditEntry{
			Agent: "relatorios", Server: "mail", Tool: "send_email",
			Decision: d,
		}, json.RawMessage(`{"body":"resumo"}`))
	}
	head := trail.Head().Hash
	_ = trail.Close()
	raw, _ := os.ReadFile(path)
	return strings.Split(strings.TrimSpace(string(raw)), "\n"), head
}

// reseal reescreve a cadeia a partir da linha from, como faria quem
// conhece o algoritmo: tudo depois dela é refeito, e a cadeia fecha.
func reseal(lines []string, from int) []string {
	var prev domain.AuditEntry
	out := make([]string, len(lines))
	for n, line := range lines {
		var e domain.AuditEntry
		_ = json.Unmarshal([]byte(line), &e)
		if n >= from {
			e.Seq, e.Prev = prev.Seq+1, prev.Hash
			if n == 0 {
				e.Prev = domain.Genesis
			}
			e = e.Seal()
			b, _ := json.Marshal(e)
			line = string(b)
		}
		out[n], prev = line, e
	}
	return out
}

// verify confere a trilha, e a cabeça, se houver, e diz em poucas
// palavras o que achou: a linha e o tipo da quebra.
func verify(lines []string, head string) string {
	report, err := audit.Verify(bytes.NewBufferString(
		strings.Join(lines, "\n") + "\n"))
	if err != nil {
		line, _, _ := strings.Cut(err.Error(), ":")
		line = strings.Replace(line, "line", "linha", 1)
		for _, k := range []struct {
			err  error
			name string
		}{
			{domain.ErrAuditHash, "hash"},
			{domain.ErrAuditPrev, "elo"},
			{domain.ErrAuditSeq, "sequência"},
			{audit.ErrLine, "forma"},
		} {
			if errors.Is(err, k.err) {
				return "acusada: " + line + ", " + k.name
			}
		}
	}
	if head != "" && report.Last.Hash != head {
		return fmt.Sprintf("acusada: a cabeça não confere (última: %d)",
			report.Last.Seq)
	}
	return "passou"
}
