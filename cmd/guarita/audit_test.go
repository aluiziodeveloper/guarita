package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	agentTrace = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	traceID    = "4bf92f3577b34da6a3ce929d0e0e4736" // o trace do agente
)

// tracedCall faz uma busca com o traceparent do agente.
func tracedCall(t *testing.T, endpoint, tool, arguments string) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` +
		`{"name":"` + tool + `","arguments":` + arguments + `}}`
	r, _ := http.NewRequest(http.MethodPost, endpoint,
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("traceparent", agentTrace)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
}

// files19 cria os caminhos da trilha e dos spans e a chave do HMAC.
func files19(t *testing.T) (trail, spans, key string) {
	t.Helper()
	dir := t.TempDir()
	key = filepath.Join(dir, "chave")
	_ = os.WriteFile(key, []byte("chave-de-laboratorio"), 0o600)
	return filepath.Join(dir, "auditoria.jsonl"),
		filepath.Join(dir, "spans.json"), key
}

// #region cmd-guarita-audit-test
// A Guarita com -audit, -audit-key e -trace-file: uma busca permitida e
// um envio negado viram duas entradas, com o trace do agente e o HMAC
// dos argumentos, sem o valor deles. Depois de parar, guarita audit
// verify confere a trilha, e o span tem o mesmo trace.
func TestRun_auditoria(t *testing.T) {
	trail, spans, key := files19(t)
	ctx, cancel := context.WithCancel(context.Background())
	endpoint, done := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-allow", "web_search", "-upstream", echoServer(t)+"/mcp",
		"-audit", trail, "-audit-key", key, "-trace-file", spans)
	tracedCall(t, endpoint, "web_search", `{"q":"trimestre"}`)
	tracedCall(t, endpoint, "send_email", `{"body":"resumo"}`)
	cancel()
	<-done
	var out bytes.Buffer
	err := run(t.Context(), []string{"audit", "verify", trail}, nil,
		nopWriteCloser{&out}, &out)
	raw, _ := os.ReadFile(trail)
	spanFile, _ := os.ReadFile(spans)
	t.Logf("%s%s", raw, out.String())
	if err != nil || strings.Count(string(raw), traceID) != 2 ||
		strings.Contains(string(raw), "trimestre") ||
		!strings.Contains(string(spanFile), traceID) {
		t.Errorf("verify: %v", err)
	}
}

// #endregion

// #region cmd-guarita-logger-test
// O registro troca os atributos de nome sensível, e os outros passam.
func TestNewLogger_redige(t *testing.T) {
	var out bytes.Buffer
	newLogger(&out).Info("pedido", "Authorization", "Bearer abc",
		"api_key", "k-123", "tool", "web_search")
	t.Log(strings.TrimSpace(out.String()))
	if strings.Contains(out.String(), "abc") ||
		strings.Contains(out.String(), "k-123") ||
		!strings.Contains(out.String(), "web_search") {
		t.Error("o registro levou um segredo")
	}
}

// #endregion
