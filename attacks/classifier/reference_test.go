package classifier

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/classifier"
	"github.com/aluiziodeveloper/guarita/internal/adapters/ollama"
	"github.com/aluiziodeveloper/guarita/internal/evaluation"
)

// maxFPR é o orçamento de falsos positivos, fixado antes da medida: no
// máximo um em vinte resultados legítimos barrado.
const maxFPR = 0.05

type example struct{ Label, Name, Text string }

// load lê um arquivo JSONL de testdata, um exemplo por linha.
func load(t *testing.T, split string) []example {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", split+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var out []example
	lines := bufio.NewScanner(file)
	for lines.Scan() {
		var e example
		if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
			t.Fatalf("%s: %v", split, err)
		}
		out = append(out, e)
	}
	return out
}

// #region attacks-classifier-live
// live liga o inspetor ao Ollama de GUARITA_OLLAMA, com o modelo de
// GUARITA_OLLAMA_MODEL, e registra o resumo do modelo: é ele, e não o
// nome, que diz qual modelo produziu os números.
func live(t *testing.T) *classifier.Inspector {
	t.Helper()
	endpoint := os.Getenv("GUARITA_OLLAMA")
	if endpoint == "" {
		t.Skip("GUARITA_OLLAMA vazio: a medida exige um Ollama local")
	}
	model := cmp.Or(os.Getenv("GUARITA_OLLAMA_MODEL"), "qwen2.5:7b")
	client, err := ollama.New(endpoint, model)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := client.Digest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("modelo %s, resumo %.12s", model, digest)
	return classifier.New(client, 0.5,
		classifier.WithTimeout(time.Minute))
}

// #endregion

// #region attacks-classifier-score
// score pontua cada exemplo com o mesmo método do inspetor e mede o
// tempo de cada pergunta.
func score(
	t *testing.T, i *classifier.Inspector, examples []example,
) ([]evaluation.Sample, []time.Duration) {
	t.Helper()
	samples := make([]evaluation.Sample, 0, len(examples))
	var latencies []time.Duration
	for _, e := range examples {
		start := time.Now()
		p, err := i.Score(t.Context(), e.Text)
		if err != nil {
			t.Fatalf("%.40s: %v", e.Text, err)
		}
		latencies = append(latencies, time.Since(start))
		positive := e.Label == "injection"
		samples = append(samples,
			evaluation.Sample{Positive: positive, Score: p})
	}
	return samples, latencies
}

// #endregion

// #region attacks-classifier-test
// O limiar sai do conjunto de desenvolvimento, com o orçamento fixado
// antes; o conjunto de teste só é pontuado depois, uma vez, com o
// limiar já escolhido. A ofuscação usa o mesmo limiar.
func TestClassificador(t *testing.T) {
	inspector := live(t)
	start := time.Now()
	_, _ = inspector.Score(t.Context(), "aquecimento")
	t.Logf("primeira pergunta: %v",
		time.Since(start).Round(time.Millisecond))
	var chosen evaluation.Point
	var latencies []time.Duration
	if !t.Run("desenvolvimento", func(t *testing.T) {
		chosen = develop(t, inspector, &latencies)
	}) {
		return // sem limiar, não há o que aplicar ao teste
	}
	t.Run("teste", func(t *testing.T) {
		check(t, inspector, load(t, "test"), chosen.Threshold,
			&latencies)
	})
	t.Run("ofuscacao", func(t *testing.T) {
		check(t, inspector, load(t, "obfuscated"), chosen.Threshold,
			&latencies)
	})
	report(t, latencies)
}

// #endregion

// #region attacks-classifier-develop
// develop varre os limiares no conjunto de desenvolvimento e escolhe o
// de maior revocação dentro do orçamento.
func develop(
	t *testing.T, i *classifier.Inspector, latencies *[]time.Duration,
) evaluation.Point {
	samples, spent := score(t, i, load(t, "dev"))
	*latencies = append(*latencies, spent...)
	points := evaluation.Sweep(samples, evaluation.Grid(0.1))
	for _, p := range points {
		t.Logf("limiar %.1f %s", p.Threshold, line(p.Confusion))
	}
	chosen, ok := evaluation.Choose(points, maxFPR)
	if !ok {
		t.Fatalf("nenhum limiar barra no máximo %.0f%% dos legítimos",
			maxFPR*100)
	}
	t.Logf("escolhido: %.1f", chosen.Threshold)
	return chosen
}

// #endregion

// #region attacks-classifier-check
// check aplica o limiar escolhido a um conjunto e lista cada erro, com
// o rótulo, a pontuação e o começo do texto.
func check(
	t *testing.T, i *classifier.Inspector, examples []example,
	threshold float64, latencies *[]time.Duration,
) {
	samples, spent := score(t, i, examples)
	*latencies = append(*latencies, spent...)
	t.Logf("limiar %.1f %s", threshold, line(evaluation.At(samples,
		threshold)))
	for n, s := range samples {
		if (s.Score >= threshold) != s.Positive {
			e := examples[n]
			t.Logf("  erro: %-9s p=%.3f %.48s", e.Label, s.Score,
				cmp.Or(e.Name, e.Text))
		}
	}
}

// #endregion

func line(c evaluation.Confusion) string {
	return fmt.Sprintf("TP=%2d FP=%2d TN=%2d FN=%2d precisão=%.2f "+
		"revocação=%.2f FPR=%.2f", c.TP, c.FP, c.TN, c.FN,
		c.Precision(), c.Recall(), c.FPR())
}

// #region attacks-classifier-report
// report resume o tempo das perguntas: a mediana, o percentil 95 e o
// pior caso. O prazo do inspetor sai daqui, e não de um palpite.
func report(t *testing.T, latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	at := func(q float64) time.Duration {
		n := int(q*float64(len(latencies))+0.999999) - 1
		return latencies[max(n, 0)].Round(time.Millisecond)
	}
	t.Logf("%d perguntas: mediana %v, p95 %v, máximo %v",
		len(latencies), at(0.5), at(0.95), at(1))
}

// #endregion
