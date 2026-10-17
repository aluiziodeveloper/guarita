package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ping(id int) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping"}`+"\n", id)
}

// nopWriteCloser deixa um bytes.Buffer servir de saída padrão.
type nopWriteCloser struct{ *bytes.Buffer }

func (nopWriteCloser) Close() error { return nil }

// #region cmd-guarita-e2e-test
// De ponta a ponta: a Guarita inicia `cat` como servidor, repassa três
// mensagens e devolve o eco ao agente.
func TestRun_repassaParaOServidor(t *testing.T) {
	input := ping(1) + ping(2) + ping(3)
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(),
		5*time.Second)
	t.Cleanup(cancel)

	err := run(ctx, []string{"--", "cat"},
		io.NopCloser(strings.NewReader(input)),
		nopWriteCloser{&stdout}, &stderr)
	if err != nil {
		t.Fatalf("run() = %v; stderr=%s", err, stderr.String())
	}
	if stdout.String() != input {
		t.Fatalf("stdout = %q; want %q", stdout.String(), input)
	}
	if !strings.Contains(stderr.String(), "3 do agente") {
		t.Fatalf("stderr = %q; want a contagem", stderr.String())
	}
}

// #endregion

func TestRun_recusaUsoInvalido(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"cat"},
		{"--"},
		{"-nope", "--", "cat"},
		{"-upstream", "http://127.0.0.1:9000/mcp", "--", "cat"},
	} {
		err := run(context.Background(), args,
			io.NopCloser(strings.NewReader("")),
			nopWriteCloser{&bytes.Buffer{}}, io.Discard)
		if !errors.Is(err, errUsage) {
			t.Errorf("run(%q) = %v; want errUsage", args, err)
		}
	}
}

func TestRun_encerraComOContexto(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	t.Cleanup(func() { _ = stdinWriter.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--", "cat"}, stdinReader,
			nopWriteCloser{&bytes.Buffer{}}, io.Discard)
	}()
	_, _ = io.WriteString(stdinWriter, ping(1))
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run() = %v; want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run não terminou depois do cancelamento")
	}
}

// #region cmd-guarita-allow-test
// Com -allow read_file, a chamada a delete_file volta ao agente como
// erro e não chega ao servidor; a permitida chega e volta pelo cat.
func TestRun_negaOQueNaoFoiPermitido(t *testing.T) {
	call := func(id int, tool string) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,`+
			`"method":"tools/call","params":{"name":%q}}`+"\n",
			id, tool)
	}
	input := call(1, "read_file") + call(2, "delete_file")
	var stdout, stderr bytes.Buffer

	err := run(context.Background(),
		[]string{"-allow", "read_file", "--", "cat"},
		io.NopCloser(strings.NewReader(input)),
		nopWriteCloser{&stdout}, &stderr)
	if err != nil {
		t.Fatalf("run() = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, call(1, "read_file")) {
		t.Fatalf("a chamada permitida não voltou do servidor: %q", out)
	}
	if strings.Contains(out, `"name":"delete_file"`) ||
		!strings.Contains(out, `"id":2,"error"`) {
		t.Fatalf("a chamada negada chegou ao servidor: %q", out)
	}
}

// #endregion

// #region cmd-guarita-http-test
// No modo HTTP, a Guarita escuta numa porta livre, repassa a chamada
// permitida, nega a outra, recusa o Host estranho e encerra com ctx.
func TestRun_servePorHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{}}`)
		}))
	t.Cleanup(upstream.Close)
	ctx, cancel := context.WithCancel(context.Background())
	endpoint, done := startHTTP(t, ctx, "-allow", "read_file",
		"-listen", "127.0.0.1:0", "-upstream", upstream.URL+"/mcp")

	expect(t, endpoint, "", toolCall(1, "read_file"), 200, `"result"`)
	expect(t, endpoint, "", toolCall(2, "delete_file"), 200, `"error"`)
	expect(t, endpoint, "evil.test", toolCall(3, "read_file"), 403,
		"host not allowed")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run() = %v; want context.Canceled", err)
		}
	case <-time.After(httpShutdownGrace):
		t.Fatal("run() não terminou depois do cancelamento")
	}
}

// #endregion

// startHTTP roda a Guarita e devolve o endpoint que ela anuncia na
// saída de erro, que também recebe o registro estruturado.
func startHTTP(
	t *testing.T, ctx context.Context, args ...string,
) (string, <-chan error) {
	t.Helper()
	stderrR, stderrW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, args, io.NopCloser(strings.NewReader("")),
			nopWriteCloser{&bytes.Buffer{}}, stderrW)
		_ = stderrW.Close()
	}()
	const prefix = "guarita: escutando em "
	lines := bufio.NewScanner(stderrR)
	for lines.Scan() {
		if address, ok := strings.CutPrefix(lines.Text(), prefix); ok {
			go func() { _, _ = io.Copy(io.Discard, stderrR) }()
			return address, done
		}
	}
	t.Fatalf("a Guarita não anunciou o endereço: %v", lines.Err())
	return "", nil
}

func toolCall(id int, tool string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call",`+
		`"params":{"name":%q}}`, id, tool)
}

func expect(
	t *testing.T, endpoint, host, body string, status int, contains string,
) {
	t.Helper()
	r, _ := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if host != "" {
		r.Host = host
	}
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	got, _ := io.ReadAll(response.Body)
	if response.StatusCode != status ||
		!strings.Contains(string(got), contains) {
		t.Fatalf("%s: %d %s; want %d com %q", body, response.StatusCode,
			got, status, contains)
	}
}
