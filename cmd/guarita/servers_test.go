package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// announces anuncia uma ferramenta com o nome dado.
func announces(tool string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":`+
			`{"tools":[{"name":%q,"description":"[LAB]"}]}}`, tool)
	}
}

// #region cmd-guarita-namespace-test
// files anuncia read_file; mail, depois, Read-File. Cada um no seu
// caminho, com o mesmo catálogo: o segundo nome é barrado.
func TestRun_namespacesPorServidor(t *testing.T) {
	files := httptest.NewServer(announces("read_file"))
	mail := httptest.NewServer(announces("Read-File"))
	t.Cleanup(files.Close)
	t.Cleanup(mail.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-catalog", filepath.Join(t.TempDir(), "catalog.db"),
		"-server", "files="+files.URL, "-server", "mail="+mail.URL)
	base := strings.TrimSuffix(endpoint, "/mcp/files")

	const list = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	fromFiles := listBody(t, base+"/mcp/files", list)
	fromMail := listBody(t, base+"/mcp/mail", list)
	t.Logf("files: %s\nmail:  %s", fromFiles, fromMail)
	if !strings.Contains(fromFiles, "read_file") ||
		strings.Contains(fromMail, "Read-File") {
		t.Fatal("o nome repetido do segundo servidor chegou ao agente")
	}
	if got := listBody(t, base+"/mcp/other", list); !strings.Contains(
		got, "unknown server") {
		t.Fatalf("servidor desconhecido: %s", got)
	}
}

// #endregion

func TestRun_serverInvalido(t *testing.T) {
	for _, value := range []string{
		"files", "=http://x", "Files=http://x",
		"a/b=http://x",
	} {
		err := run(context.Background(), []string{"-server", value},
			io.NopCloser(strings.NewReader("")),
			nopWriteCloser{nil}, io.Discard)
		if !errors.Is(err, errUsage) {
			t.Errorf("-server %q: %v", value, err)
		}
	}
}

// recorder responde e guarda o Authorization que recebeu.
func recorder(seen chan<- string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}
}

// #region cmd-guarita-credentials-test
// Só files tem credencial. Ela não pode chegar a mail.
func TestRun_credencialPorServidor(t *testing.T) {
	fromFiles, fromMail := make(chan string, 1), make(chan string, 1)
	files := httptest.NewServer(recorder(fromFiles))
	mail := httptest.NewServer(recorder(fromMail))
	t.Cleanup(files.Close)
	t.Cleanup(mail.Close)
	cred := filepath.Join(t.TempDir(), "files.cred")
	_ = os.WriteFile(cred, []byte("cred-de-files\n"), 0o600)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-server", "files="+files.URL, "-server", "mail="+mail.URL,
		"-server-credential", "files="+cred)
	base := strings.TrimSuffix(endpoint, "/mcp/files")
	const ping = `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	_ = listBody(t, base+"/mcp/files", ping)
	_ = listBody(t, base+"/mcp/mail", ping)
	gotFiles, gotMail := <-fromFiles, <-fromMail
	t.Logf("files recebeu %q; mail recebeu %q", gotFiles, gotMail)
	if gotFiles != "Bearer cred-de-files" || gotMail != "" {
		t.Fatal("a credencial foi para o servidor errado")
	}
}

// #endregion

func TestRun_credencialSemDono(t *testing.T) {
	for _, args := range [][]string{
		{"-server", "files=http://x", "-upstream-credential", "c"},
		{"-server", "files=http://x", "-server-credential", "mail=c"},
		{"-server", "files=http://x", "-server-credential", "files"},
	} {
		err := run(context.Background(), args,
			io.NopCloser(strings.NewReader("")),
			nopWriteCloser{nil}, io.Discard)
		if !errors.Is(err, errUsage) {
			t.Errorf("%v: %v", args, err)
		}
	}
}
