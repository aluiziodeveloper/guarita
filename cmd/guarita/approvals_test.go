package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const anaSecret = "segredo-da-ana-0123456789"

// operatorFiles grava o arquivo dos operadores, para a Guarita, e o
// segredo da ana, para a linha de comando, com permissão 0600.
func operatorFiles(t *testing.T) (operators, ana string) {
	t.Helper()
	dir := t.TempDir()
	operators = filepath.Join(dir, "operators")
	ana = filepath.Join(dir, "ana.secret")
	for path, content := range map[string]string{
		operators: "ana " + anaSecret + "\n", ana: anaSecret + "\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return operators, ana
}

// startWithAdmin sobe a Guarita e devolve os endereços do agente e do
// operador, lidos das linhas que ela escreve ao subir.
func startWithAdmin(
	t *testing.T, ctx context.Context, args ...string,
) (agent, operator string) {
	t.Helper()
	stderrR, stderrW := io.Pipe()
	go func() {
		_ = run(ctx, args, io.NopCloser(strings.NewReader("")),
			nopWriteCloser{&bytes.Buffer{}}, stderrW)
		_ = stderrW.Close()
	}()
	lines := bufio.NewScanner(stderrR)
	for (agent == "" || operator == "") && lines.Scan() {
		line := lines.Text()
		if a, ok := strings.CutPrefix(line, "guarita: aprovações em "); ok {
			operator = strings.TrimSuffix(a, "/")
		}
		if a, ok := strings.CutPrefix(line, "guarita: escutando em "); ok {
			agent = a
		}
	}
	go func() { _, _ = io.Copy(io.Discard, stderrR) }()
	return agent, operator
}

// #region cmd-guarita-approvals-test
// manutencao pede delete_database; a política pede aprovação; a ana
// lista pela linha de comando e aprova; só então a chamada segue.
func TestRun_aprovacaoPelaLinhaDeComando(t *testing.T) {
	upstream, reached := countingServer(t)
	pub, _, key := labFiles(t)
	operators, ana := operatorFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	agent, operator := startWithAdmin(t, ctx, "-listen", "127.0.0.1:0",
		"-policy", "../../policies", "-agent-key", pub,
		"-agent-issuer", labIssuer, "-server", "db="+upstream,
		"-operators", operators, "-admin-listen", "127.0.0.1:0")
	token := tokenFor(t, key, agent, "manutencao")
	answer := make(chan string, 1)
	go func() { answer <- callAs(t, agent, token, "delete_database") }()
	cli := []string{"-admin", operator, "-secret", ana}
	list := waitForPending(t, ctx, cli)
	t.Logf("guarita approvals list:\n%s", list)
	id := strings.Fields(strings.Split(list, "\n")[1])[0]
	err := run(ctx, append([]string{"approvals"},
		append(cli, "approve", id)...), nil, nopWriteCloser{nil},
		io.Discard)
	t.Logf("approve: %v; o agente recebe %s", err, <-answer)
	if err != nil || len(reached) != 1 {
		t.Fatalf("o servidor recebeu %d chamadas", len(reached))
	}
}

// #endregion

// waitForPending lista até aparecer um pedido; a chamada do agente
// chega à fila numa goroutine, sem aviso.
func waitForPending(
	t *testing.T, ctx context.Context, cli []string,
) string {
	t.Helper()
	for range 100 {
		var out bytes.Buffer
		args := append([]string{"approvals"}, append(cli, "list")...)
		err := run(ctx, args, nil, nopWriteCloser{&out}, io.Discard)
		if err == nil && strings.Count(out.String(), "\n") > 1 {
			return strings.TrimSpace(out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("o pedido não chegou à fila")
	return ""
}

func TestRun_operadorSoEmLoopback(t *testing.T) {
	operators, _ := operatorFiles(t)
	err := run(context.Background(), []string{
		"-listen", "127.0.0.1:0",
		"-upstream", "http://127.0.0.1:1/mcp", "-operators", operators,
		"-admin-listen", "0.0.0.0:0",
	}, nil, nopWriteCloser{nil}, io.Discard)
	t.Logf("-admin-listen 0.0.0.0:0: %v", err)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("run() = %v", err)
	}
}
