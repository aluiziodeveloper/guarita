package domain

import (
	"errors"
	"testing"
	"time"
)

// chain monta três entradas encadeadas.
func chain() []AuditEntry {
	at := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	var out []AuditEntry
	prev := AuditEntry{}
	for n, d := range []Decision{
		DecisionAllow, DecisionDeny, DecisionAllow,
	} {
		e := AuditEntry{
			Seq: uint64(n + 1), Time: at.Add(time.Duration(n) * time.Second),
			Agent: "relatorios", Server: "web", Tool: "web_search",
			Decision: d, Prev: Genesis,
		}
		if prev.Hash != "" {
			e.Prev = prev.Hash
		}
		prev = e.Seal()
		out = append(out, prev)
	}
	return out
}

// #region domain-audit-test
// A cadeia intacta confere elo por elo. Trocar a decisão da segunda
// entrada quebra o hash dela; recalcular o hash da segunda, para
// esconder a troca, quebra o elo da terceira, que guarda o hash antigo.
func TestFollows(t *testing.T) {
	entries := chain()
	for n := range entries {
		prev := AuditEntry{}
		if n > 0 {
			prev = entries[n-1]
		}
		if err := entries[n].Follows(prev); err != nil {
			t.Fatalf("cadeia intacta: %v", err)
		}
	}
	edited := entries[1]
	edited.Decision = DecisionAllow
	err1 := edited.Follows(entries[0])
	resealed := edited.Seal()
	err2 := entries[2].Follows(resealed)
	t.Logf("decisão trocada: %v", err1)
	t.Logf("hash recalculado: %v", err2)
	if !errors.Is(err1, ErrAuditHash) ||
		!errors.Is(err2, ErrAuditPrev) {
		t.Errorf("%v / %v", err1, err2)
	}
}

// #endregion

func TestFollows_sequencia(t *testing.T) {
	entries := chain()
	if err := entries[2].Follows(entries[0]); !errors.Is(err,
		ErrAuditSeq) {
		t.Errorf("entrada removida: %v", err)
	}
}
