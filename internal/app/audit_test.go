package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// memoryAuditor guarda as entradas; com fail, recusa todas.
type memoryAuditor struct {
	entries []domain.AuditEntry
	fail    error
}

func (a *memoryAuditor) Record(
	_ context.Context, e domain.AuditEntry, _ json.RawMessage,
) error {
	if a.fail != nil {
		return a.fail
	}
	a.entries = append(a.entries, e)
	return nil
}

// #region app-audit-test
// Uma chamada permitida e uma negada viram duas entradas, com o motivo
// da negação. Com a trilha fora do ar, a chamada que seria permitida é
// negada: sem registro, a Guarita não age.
func TestAuthorize_audita(t *testing.T) {
	call := domain.Call{ToolName: "read_file", Arguments: []byte(`{}`)}
	for _, d := range []domain.Decision{
		domain.DecisionAllow, domain.DecisionDeny,
	} {
		auditor := &memoryAuditor{}
		uc := NewInterceptCall(&stubPolicy{decision: d}, nil,
			WithServer("files"), WithAuditor(auditor))
		err := uc.Authorize(t.Context(), call)
		if len(auditor.entries) != 1 ||
			auditor.entries[0].Decision != d {
			t.Fatalf("%s: %+v", d, auditor.entries)
		}
		e := auditor.entries[0]
		t.Logf("%-5s -> %s/%s %s %q (erro: %v)", d, e.Server, e.Tool,
			e.Decision, e.Reason, err)
	}
	broken := &memoryAuditor{fail: errors.New("disco cheio")}
	uc := NewInterceptCall(&stubPolicy{decision: domain.DecisionAllow},
		nil, WithAuditor(broken))
	err := uc.Authorize(t.Context(), call)
	t.Logf("trilha fora: %v", err)
	if !errors.Is(err, ErrAuditFailed) {
		t.Errorf("permitida sem registro: %v", err)
	}
}

// #endregion
