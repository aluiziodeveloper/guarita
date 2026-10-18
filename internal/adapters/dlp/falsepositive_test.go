package dlp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"testing"
)

// benign são geradores de valores legítimos que aparecem em
// argumentos de ferramenta e que um detector pode confundir.
var benign = []struct {
	name     string
	generate func(*rand.Rand) string
}{
	{"número de pedido, 11 dígitos", func(r *rand.Rand) string {
		return fmt.Sprintf("%011d", r.Int64N(100_000_000_000))
	}},
	{"telefone celular", func(r *rand.Rand) string {
		return fmt.Sprintf("(11) 9%04d-%04d", r.IntN(10000), r.IntN(10000))
	}},
	{"UUID", func(r *rand.Rand) string {
		return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x", r.Uint32(),
			r.IntN(1<<16), r.IntN(1<<12), r.IntN(1<<16), r.Int64N(1<<48))
	}},
	{"SHA-256 em hexadecimal", func(r *rand.Rand) string {
		sum := sha256.Sum256(fmt.Appendf(nil, "%d", r.Int64()))
		return hex.EncodeToString(sum[:])
	}},
	{"caminho de arquivo", func(r *rand.Rand) string {
		return fmt.Sprintf("reports/2026/q%d/resumo_%03d.txt",
			r.IntN(4)+1, r.IntN(1000))
	}},
	{"miniatura em base64, 32 bytes", func(r *rand.Rand) string {
		blob := make([]byte, 32)
		for i := range blob {
			blob[i] = byte(r.IntN(256))
		}
		return base64.StdEncoding.EncodeToString(blob)
	}},
}

// #region dlp-false-positives-test
// Mil valores legítimos de cada tipo, gerados com semente fixa: quantos
// cada detector acusa. A taxa medida é o custo da regra, e entra na
// tabela de decisões como argumento, não como palpite.
func TestFalsosPositivos(t *testing.T) {
	for _, kind := range benign {
		r := rand.New(rand.NewPCG(13, 2026))
		counts := map[string]int{}
		for range 1000 {
			for _, f := range inspectText(kind.generate(r)).Findings {
				counts[f.Signal]++
			}
		}
		t.Logf("%-30s acusados em 1000: %v", kind.name, counts)
		if counts[SignalSecret] > 0 || counts[SignalCPF] > 20 {
			t.Errorf("%s: falsos positivos acima do medido", kind.name)
		}
	}
}

// #endregion
