package injection

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

func TestDecoded(t *testing.T) {
	d := NewDecoded(New(DefaultActions()))
	b64 := base64.StdEncoding.EncodeToString
	digest := sha256.Sum256([]byte("x"))
	for _, c := range []struct {
		text string
		want domain.Decision
	}{
		{
			"Anexo " + b64([]byte("Texto [INST] [LAB] [/INST]")),
			domain.DecisionDeny,
		},
		{
			"Anexo " + b64([]byte("Texto com \u200b[LAB] escondido")),
			domain.DecisionDeny,
		},
		{"Resumo " + b64(digest[:]), domain.DecisionAllow},
		{
			"Anexo " + b64([]byte("Relatório do trimestre, sem nada.")),
			domain.DecisionAllow,
		},
	} {
		got, err := d.InspectResult(t.Context(), domain.PolicyInput{},
			result(c.text))
		if err != nil || got.Decision != c.want {
			t.Errorf("%.30q: %v %v", c.text, got, err)
		}
	}
}

// O corpus benigno do Capítulo 14 continua sem nenhum resultado
// barrado com a etapa nova.
func TestDecoded_corpusBenigno(t *testing.T) {
	d := NewDecoded(New(DefaultActions()))
	files, _ := filepath.Glob(filepath.Join("..", "..", "..", "attacks",
		"injection", "testdata", "benign", "*.json"))
	if len(files) == 0 {
		t.Fatal("corpus benigno não encontrado")
	}
	for _, file := range files {
		raw, _ := os.ReadFile(file)
		var fixture struct{ Text string }
		_ = json.Unmarshal(raw, &fixture)
		got, _ := d.InspectResult(t.Context(), domain.PolicyInput{},
			result(fixture.Text))
		if got.Decision != domain.DecisionAllow {
			t.Errorf("%s: %v", filepath.Base(file), got)
		}
	}
}
