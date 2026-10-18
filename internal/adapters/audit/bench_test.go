package audit

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region audit-benchmark
// BenchmarkRecord mede uma entrada inteira: numerar, calcular o HMAC e
// o SHA-256, escrever a linha e esperar o fsync. O disco é o do
// diretório temporário da máquina que roda o teste.
func BenchmarkRecord(b *testing.B) {
	trail, err := Open(filepath.Join(b.TempDir(), "a.jsonl"),
		[]byte("chave"))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = trail.Close() }()
	e := domain.AuditEntry{
		Agent: "relatorios", Server: "web",
		Tool: "web_search", Decision: domain.DecisionAllow,
	}
	args := json.RawMessage(`{"q":"relatório do trimestre"}`)
	for b.Loop() {
		_ = trail.Record(b.Context(), e, args)
	}
}

// #endregion
