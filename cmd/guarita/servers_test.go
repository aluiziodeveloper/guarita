package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
