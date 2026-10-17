package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// callAs faz um tools/call no caminho dado com o token do agente e
// devolve o corpo da resposta.
func callAs(t *testing.T, endpoint, token, tool string) string {
	t.Helper()
	r, _ := http.NewRequest("POST", endpoint,
		strings.NewReader(toolCall(1, tool)))
	r.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	return strings.TrimSpace(string(body))
}

// countingServer responde a todo pedido e anota o caminho de cada um.
func countingServer(t *testing.T) (string, chan string) {
	t.Helper()
	reached := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			reached <- r.URL.Path
			_, _ = io.WriteString(w,
				`{"jsonrpc":"2.0","id":1,"result":{}}`)
		}))
	t.Cleanup(server.Close)
	return server.URL, reached
}

// #region cmd-guarita-policy-test
// A política versionada decide com o agente do token e o servidor da
// rota: relatorios lê em files e não apaga em db; manutencao precisa
// de aprovação para apagar.
func TestRun_politicaPorAgenteEServidor(t *testing.T) {
	upstream, reached := countingServer(t)
	pub, _, key := labFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-policy", "../../policies", "-agent-key", pub,
		"-agent-issuer", labIssuer,
		"-server", "files="+upstream+"/files",
		"-server", "db="+upstream+"/db")
	base := strings.TrimSuffix(endpoint, "/mcp/files")
	for _, c := range []struct{ agent, server, tool string }{
		{"relatorios", "files", "read_file"},
		{"relatorios", "db", "delete_database"},
		{"manutencao", "db", "delete_database"},
	} {
		url := base + "/mcp/" + c.server
		body := callAs(t, url, tokenFor(t, key, url, c.agent), c.tool)
		t.Logf("%-10s %-5s %-15s %s", c.agent, c.server, c.tool, body)
	}
	if got := len(reached); got != 1 {
		t.Fatalf("o servidor recebeu %d chamadas, e não 1", got)
	}
}

// #endregion

func TestRun_politicaEListaJuntasSaoErro(t *testing.T) {
	err := run(context.Background(), []string{
		"-policy", "../../policies",
		"-allow", "read_file", "--", "true",
	},
		io.NopCloser(strings.NewReader("")), nopWriteCloser{nil}, io.Discard)
	if !errors.Is(err, errPolicyAndAllow) {
		t.Fatalf("run() = %v", err)
	}
}

func TestRun_politicaInvalidaImpedeAPartida(t *testing.T) {
	dir := t.TempDir()
	err := run(context.Background(), []string{
		"-policy", dir,
		"--", "true",
	},
		io.NopCloser(strings.NewReader("")), nopWriteCloser{nil}, io.Discard)
	t.Logf("diretório vazio: %v", err)
	if err == nil {
		t.Fatal("a Guarita não deveria partir sem política")
	}
}
