package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// probe faz quatro buscas e confere o status e o veredito de cada uma.
func probe(t *testing.T, name, endpoint string, want []string) {
	t.Helper()
	for n, w := range want {
		status, got := statusOf(t, endpoint)
		line := fmt.Sprintf("%d %s", status, got)
		t.Logf("%-9s chamada %d: %.66s", name, n+1, line)
		if !strings.HasPrefix(line, w) {
			t.Errorf("%s %d: %s; quer %s", name, n+1, line, w)
		}
	}
}

// #region cmd-guarita-limits-test
// -quota web_search=3/m: a quarta busca seguida do mesmo agente é
// negada, como toda negação, num erro JSON-RPC. -breaker-failures 2,
// com um servidor que responde 502: a terceira chamada nem chega a
// ele e volta com 503.
func TestRun_limites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	echo, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-allow", "web_search", "-upstream", echoServer(t)+"/mcp",
		"-quota", "web_search=3/m")
	probe(t, "cota", echo, []string{
		"200 passou", "200 passou",
		"200 passou", "200 negado",
	})
	failing := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
	t.Cleanup(failing.Close)
	down, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-allow", "web_search", "-upstream", failing.URL+"/mcp",
		"-breaker-failures", "2")
	probe(t, "disjuntor", down, []string{
		"502", "502", "503 negado",
		"503 negado",
	})
}

// #endregion

// statusOf faz uma busca e devolve o status HTTP e o veredito.
func statusOf(t *testing.T, endpoint string) (int, string) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` +
		`{"name":"web_search","arguments":{"q":"relatório"}}}`
	r, err := http.Post(endpoint, "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	reply, _ := io.ReadAll(r.Body)
	return r.StatusCode, verdict(string(reply))
}

// #endregion

// Um custo maior que a rajada do orçamento nunca caberia.
func TestRun_custoMaiorQueOOrcamento(t *testing.T) {
	err := run(t.Context(), []string{
		"-listen", "127.0.0.1:0",
		"-allow", "x", "-upstream", "http://127.0.0.1:1/mcp",
		"-budget", "100/h", "-cost", "ask_model=200",
	}, nil, nil,
		&strings.Builder{})
	if !errors.Is(err, errLimits) {
		t.Errorf("subiu: %v", err)
	}
}
