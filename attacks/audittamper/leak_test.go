package audittamper

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/examples/naiveaudit"
	"github.com/aluiziodeveloper/guarita/internal/adapters/audit"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region attacks-audittamper-leak-test
// O mesmo envio, com o CPF de exemplo, pelo registro ingênuo e pela
// trilha. O ingênuo guarda o CPF; a trilha guarda o HMAC, e quem tem a
// chave e o argumento suspeito confere que foi ele.
func TestVazamento(t *testing.T) {
	cpf := "123456789" + "09" // o CPF de exemplo das documentações
	args := `{"body":"cliente ` + cpf + `"}`
	var naive bytes.Buffer
	naiveaudit.Logger(&naive)("relatorios", "send_email", "deny", args)
	path := filepath.Join(t.TempDir(), "auditoria.jsonl")
	key := []byte("chave-de-laboratorio")
	trail, _ := audit.Open(path, key)
	_ = trail.Record(t.Context(), domain.AuditEntry{
		Agent: "relatorios", Tool: "send_email", Decision: "deny",
	}, json.RawMessage(args))
	mac := trail.Head().ArgsMAC
	_ = trail.Close()
	raw, _ := os.ReadFile(path)
	again := audit.MAC(key, json.RawMessage(args))
	t.Logf("CPF no ingênuo: %v; CPF na trilha: %v; HMAC confere: %v",
		strings.Contains(naive.String(), cpf),
		strings.Contains(string(raw), cpf), again == mac)
	if !strings.Contains(naive.String(), cpf) ||
		strings.Contains(string(raw), cpf) || again != mac {
		t.Error("o registro mudou de comportamento")
	}
}

// #endregion
