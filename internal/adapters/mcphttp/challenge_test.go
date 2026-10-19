package mcphttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// #region mcphttp-challenge-test
// O desafio do Capítulo 22: no meio da resposta de tools/call, o
// servidor manda duas notificações de progresso, uma com [LAB] na
// mensagem e outra limpa. Com WithResults, a barrada não chega ao
// agente; a limpa chega como veio, e a resposta, revisada.
func TestHandler_revisaAsNotificacoes_desafio(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			for _, text := range []string{"[LAB] ordem", "metade"} {
				event(w, map[string]any{
					"jsonrpc": "2.0",
					"method":  "notifications/progress",
					"params": map[string]any{
						"progressToken": "t1",
						"progress":      1, "message": text,
					},
				})
			}
			event(w, map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []map[string]string{
					{"type": "text", "text": "pronto"},
				}},
			})
		}))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "web_search"),
		WithResults(labReviewer{}))
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)
	res, err := http.Post(guarita.URL+"/mcp", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,`+
			`"method":"tools/call","params":{"name":"web_search",`+
			`"arguments":{"q":"x"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	t.Logf("o agente recebe:\n%s", body)
	for _, c := range []struct {
		text string
		want bool
	}{
		{"[LAB] ordem", false},
		{"metade", true},
		{"[de web_search] ok", true},
	} {
		if strings.Contains(string(body), c.text) != c.want {
			t.Errorf("%q no fluxo? quer %v", c.text, c.want)
		}
	}
}

// #endregion

// event escreve uma mensagem como um evento SSE.
func event(w io.Writer, message map[string]any) {
	data, _ := json.Marshal(message)
	_, _ = io.WriteString(w,
		"event: message\ndata: "+string(data)+"\n\n")
}
