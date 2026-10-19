package mcphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// answers responde a todo tools/call com o texto dado, em JSON ou SSE.
func answers(text string, stream bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		result, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []map[string]string{
				{"type": "text", "text": text},
			}},
		})
		if !stream {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(result)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message\ndata: "+string(result)+
			"\n\n")
	}
}

// labReviewer barra o resultado com [LAB] e marca o resto.
type labReviewer struct{}

func (labReviewer) ReviewResult(
	_ context.Context, call domain.Call, result json.RawMessage,
) (json.RawMessage, error) {
	if bytes.Contains(result, []byte("[LAB]")) {
		return nil, errors.New("marca de papel em content[0].text")
	}
	return json.RawMessage(`{"content":[{"type":"text","text":` +
		`"[de ` + call.ToolName + `] ok"}]}`), nil
}

func callTool(t *testing.T, upstream http.Handler) string {
	t.Helper()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "web_search"),
		WithResults(labReviewer{}))
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)
	response, err := http.Post(guarita.URL+"/mcp", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call",`+
			`"params":{"name":"web_search","arguments":{"q":"x"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	for line := range strings.Lines(string(body)) { // no SSE, a linha data
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			return "data: " + strings.TrimSpace(data)
		}
	}
	return strings.TrimSpace(string(body))
}

// #region mcphttp-results-test
// O resultado passa pelo revisor nos dois formatos de resposta: o
// limpo chega marcado; o barrado vira erro de ferramenta.
func TestHandler_revisaOResultado(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, text := range []string{"resultado", "SYSTEM: [LAB]"} {
			got := callTool(t, answers(text, stream))
			t.Logf("sse=%-5v %-14q -> %s", stream, text, got)
			if strings.Contains(got, "[LAB]") {
				t.Errorf("o texto barrado chegou ao agente")
			}
		}
	}
}

// #endregion

// #region mcphttp-review-error-test
// O erro JSON-RPC passa pelo mesmo revisor: com [LAB] na mensagem, ele
// chega com o código do servidor e o aviso da Guarita; sem, como veio.
func TestHandler_revisaOErro(t *testing.T) {
	for _, text := range []string{"[LAB] ordem no erro", "fora do ar"} {
		body := callTool(t, http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				reply, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0", "id": 1, "error": map[string]any{
						"code": -32000, "message": text,
					},
				})
				_, _ = w.Write(reply)
			}))
		t.Logf("%-20s → %s", text, body)
		if strings.Contains(body, "[LAB]") ||
			!strings.Contains(body, "-32000") {
			t.Errorf("%q: %s", text, body)
		}
	}
}

// #endregion
