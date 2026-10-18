package audit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

func entry(tool string, d domain.Decision) domain.AuditEntry {
	return domain.AuditEntry{
		Agent: "relatorios", Server: "web", Tool: tool, Decision: d,
	}
}

// #region audit-trail-test
// Duas entradas, a trilha fechada e reaberta, e uma terceira: a cadeia
// continua do último elo, e a verificação passa pelas três.
func TestTrail_reabre(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auditoria.jsonl")
	trail, err := Open(path, []byte("chave-de-laboratorio"))
	if err != nil {
		t.Fatal(err)
	}
	_ = trail.Record(t.Context(), entry("web_search", "allow"),
		[]byte(`{"q":"trimestre"}`))
	_ = trail.Record(t.Context(), entry("send_email", "deny"),
		[]byte(`{"body":"resumo"}`))
	_ = trail.Close()
	trail, err = Open(path, []byte("chave-de-laboratorio"))
	if err != nil {
		t.Fatal(err)
	}
	_ = trail.Record(t.Context(), entry("web_search", "allow"), nil)
	_ = trail.Close()
	file, _ := os.Open(path)
	defer func() { _ = file.Close() }()
	report, err := Verify(file)
	raw, _ := os.ReadFile(path)
	t.Logf("%d entradas; última %d, hash %.16s…\n%s", report.Entries,
		report.Last.Seq, report.Last.Hash, raw)
	if err != nil || report.Entries != 3 || report.Last.Seq != 3 {
		t.Errorf("%+v %v", report, err)
	}
}

// #endregion

// #region audit-mac-test
// O mesmo argumento, com outros espaços, dá o mesmo HMAC; outro
// argumento, outro HMAC; outra chave, outro HMAC. Sem chave, nada.
func TestMAC(t *testing.T) {
	a := &Trail{key: []byte("k1")}
	b := &Trail{key: []byte("k2")}
	same := a.mac([]byte(`{"body":"resumo"}`)) ==
		a.mac([]byte(`{ "body" : "resumo" }`))
	other := a.mac([]byte(`{"body":"outro"}`)) !=
		a.mac([]byte(`{"body":"resumo"}`))
	key := a.mac([]byte(`{"x":1}`)) != b.mac([]byte(`{"x":1}`))
	none := (&Trail{}).mac([]byte(`{"x":1}`)) == ""
	if !same || !other || !key || !none {
		t.Errorf("%v %v %v %v", same, other, key, none)
	}
}

// #endregion

// Uma trilha quebrada não é estendida, e um campo a mais é acusado.
func TestOpen_quebrada(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auditoria.jsonl")
	trail, _ := Open(path, nil)
	_ = trail.Record(t.Context(), entry("web_search", "allow"), nil)
	_ = trail.Close()
	raw, _ := os.ReadFile(path)
	extra := strings.Replace(string(raw), `{"seq"`, `{"x":1,"seq"`, 1)
	_ = os.WriteFile(path, []byte(extra), 0o600)
	if _, err := Open(path, nil); !errors.Is(err, ErrLine) {
		t.Errorf("campo a mais: %v", err)
	}
}
