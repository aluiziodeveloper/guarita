package mcphttp

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// recorder guarda os cabeçalhos que chegaram ao servidor.
func recorder(seen chan<- http.Header) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		seen <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}
}

// O agente manda o token do usuário em quatro lugares. Nenhum chega;
// os cabeçalhos do MCP chegam.
func TestHandler_repassaSoOsCabecalhosPermitidos(t *testing.T) {
	seen := make(chan http.Header, 1)
	upstream := httptest.NewServer(recorder(seen))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "read_file"))
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)
	// #region mcphttp-allowed-headers-test
	const secret = "tok-do-usuario"
	r, _ := http.NewRequest("POST", guarita.URL,
		strings.NewReader(toolCall(1, "read_file")))
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("X-Api-Key", secret)
	r.Header.Set("Cookie", "session="+secret)
	r.Header.Set("Proxy-Authorization", "Basic "+secret)
	r.Header.Set("Mcp-Param-Region", "sa-east-1")
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = response.Body.Close()

	got := <-seen
	t.Logf("o servidor recebeu: %v", slices.Sorted(maps.Keys(got)))
	for name, values := range got {
		if strings.Contains(strings.Join(values, ","), secret) {
			t.Errorf("o token do usuário chegou em %s", name)
		}
	}
	if got.Get("Mcp-Param-Region") != "sa-east-1" {
		t.Errorf("Mcp-Param-Region não chegou")
	}
}

// #endregion
