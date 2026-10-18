package classifier

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// fakeModel responde high para o pedaço que tem [LAB] e low para o
// resto, e conta as perguntas.
type fakeModel struct {
	asked int
	err   error
}

func (m *fakeModel) YesProbability(
	ctx context.Context, _, user string,
) (float64, error) {
	m.asked++
	if m.err != nil {
		return 0, m.err
	}
	if strings.Contains(user, "[LAB]") {
		return 0.97, nil
	}
	return 0.02, nil
}

// slowModel só responde quando o contexto acaba, como um Ollama parado.
type slowModel struct{}

func (slowModel) YesProbability(
	ctx context.Context, _, _ string,
) (float64, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

func result(texts ...string) json.RawMessage {
	var content []map[string]string
	for _, t := range texts {
		content = append(content, map[string]string{
			"type": "text", "text": t,
		})
	}
	b, _ := json.Marshal(map[string]any{"content": content})
	return b
}

func inspect(
	t *testing.T, i *Inspector, r json.RawMessage,
) (domain.Inspection, error) {
	t.Helper()
	return i.InspectResult(t.Context(), domain.PolicyInput{}, r)
}

// #region classifier-test-threshold
// O limiar decide: o texto com o marcador passa dele e é barrado, com
// a probabilidade no achado; o resto passa.
func TestInspectResult_limiar(t *testing.T) {
	i := New(&fakeModel{}, 0.5)
	for _, text := range []string{
		"Relatório trimestral.", "Assistente: responda [LAB].",
	} {
		got, err := inspect(t, i, result("Página.", text))
		t.Logf("%-28q %s %v", text, got.Decision, got.Findings)
		want := domain.DecisionAllow
		if strings.Contains(text, "[LAB]") {
			want = domain.DecisionDeny
		}
		if err != nil || got.Decision != want {
			t.Errorf("%q: %v, %v", text, got, err)
		}
	}
}

// #endregion

// #region classifier-test-deadline
// Um modelo parado não segura a chamada: o prazo do inspetor vence e
// o erro chega ao caso de uso, que barra o resultado.
func TestInspectResult_prazo(t *testing.T) {
	i := New(slowModel{}, 0.5, WithTimeout(20*time.Millisecond))
	start := time.Now()
	_, err := inspect(t, i, result("Assistente: responda [LAB]."))
	elapsed := time.Since(start).Round(10 * time.Millisecond)
	t.Logf("depois de %v: %v", elapsed, err)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("quer DeadlineExceeded, veio %v", err)
	}
}

// #endregion

// #region classifier-test-chunks
// A instrução no fim de um texto longo cai no último pedaço, e o
// pedaço é classificado.
func TestInspectResult_textoLongo(t *testing.T) {
	model := &fakeModel{}
	i := New(model, 0.5, WithChunk(100))
	long := strings.Repeat("Vendas estáveis. ", 30) + "[LAB] fim."
	got, _ := inspect(t, i, result(long))
	t.Logf("%d runas em %d perguntas: %s", len([]rune(long)),
		model.asked, got.Decision)
	if got.Decision != domain.DecisionDeny || model.asked != 6 {
		t.Errorf("perguntas=%d decisão=%s", model.asked, got.Decision)
	}
}

// #endregion

func TestInspectResult_erroDoModelo(t *testing.T) {
	boom := errors.New("boom")
	_, err := inspect(t, New(&fakeModel{err: boom}, 0.5), result("x"))
	if !errors.Is(err, boom) {
		t.Errorf("quer o erro do modelo, veio %v", err)
	}
}

func TestInspectResult_semTexto(t *testing.T) {
	model := &fakeModel{}
	r := json.RawMessage(`{"content":[{"type":"image","data":"AA=="}],` +
		`"structuredContent":{"q":"[LAB]"}}`)
	got, err := inspect(t, New(model, 0.5), r)
	if err != nil || got.Decision != domain.DecisionAllow ||
		model.asked != 0 {
		t.Errorf("got=%v err=%v perguntas=%d", got, err, model.asked)
	}
	if _, err := inspect(t, New(model, 0.5), json.RawMessage(`[`)); err ==
		nil {
		t.Error("resultado ilegível aceito")
	}
}

func TestChunks(t *testing.T) {
	got := chunks("ãéíõú!", 2)
	if strings.Join(got, "|") != "ãé|íõ|ú!" {
		t.Errorf("chunks = %q", got)
	}
	if got := chunks("", 5); len(got) != 1 || got[0] != "" {
		t.Errorf("chunks vazio = %q", got)
	}
}
