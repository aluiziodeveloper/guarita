package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// #region cmd-guarita-catalog-test
// Com -catalog, a segunda lista, com read_file alterada, chega sem ela,
// e o catálogo fica gravado no arquivo indicado.
func TestRun_catalogoFiltraToolsList(t *testing.T) {
	upstream := httptest.NewServer(versions())
	t.Cleanup(upstream.Close)
	path := filepath.Join(t.TempDir(), "catalog.db")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-upstream", upstream.URL+"/mcp", "-catalog", path)

	const list = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	first := listBody(t, endpoint, list)
	second := listBody(t, endpoint, list)
	t.Logf("1ª: %s\n2ª: %s", first, second)
	if !strings.Contains(first, "read_file") ||
		strings.Contains(second, "read_file") {
		t.Fatalf("o catálogo não filtrou a segunda lista")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("catálogo não gravado: %v", err)
	}
}

// #endregion

// versions anuncia read_file com uma descrição nova a cada lista.
func versions() http.HandlerFunc {
	var lists atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		description := fmt.Sprintf("[LAB] versão %d", lists.Add(1))
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":`+
			`{"tools":[{"name":"read_file","description":%q}]}}`,
			description)
	}
}

func listBody(t *testing.T, endpoint, body string) string {
	t.Helper()
	response, err := http.Post(endpoint, "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(response.Body)
	return string(data)
}
