package shadowing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/adapters/scanner"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sqlitestore"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// behind põe o servidor atrás de um handler da Guarita, com o catálogo
// do namespace dado se store não for nil, e devolve uma sessão.
func behind(
	t *testing.T, server *mcp.Server, namespace string,
	store app.CatalogStore,
) *mcp.ClientSession {
	t.Helper()
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(upstream.Close)
	policy, _ := policystatic.New(map[string]domain.Decision{
		"read_file": domain.DecisionAllow, "Read-File": domain.DecisionAllow,
	})
	var useCase []app.Option
	var handler []mcphttp.Option
	if store != nil {
		catalog := app.NewPinCatalog(store, namespace,
			app.WithScanner(scanner.Structural{}))
		useCase = append(useCase, app.WithCatalog(catalog))
		handler = append(handler, mcphttp.WithCatalog(catalog))
	}
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil, useCase...))
	target, _ := url.Parse(upstream.URL + "/mcp")
	proxy, _ := mcphttp.NewHandler(target, gate, handler...)
	guarita := httptest.NewServer(proxy)
	t.Cleanup(guarita.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "agent"}, nil).
		Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint: guarita.URL + "/mcp",
		}, nil)
	if err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// sharedStore abre o catálogo dos dois servidores, ou nil sem defesa.
func sharedStore(t *testing.T, defended bool) app.CatalogStore {
	t.Helper()
	if !defended {
		return nil
	}
	store, err := sqlitestore.Open(context.Background(),
		filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() = %v", err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// #region attacks-shadowing-test
// O agente usa os dois servidores. Sem catálogo, vê read_file e
// Read-File. Com o catálogo compartilhado, Read-File some e é negada.
func TestShadowing(t *testing.T) {
	for _, defended := range []bool{false, true} {
		store := sharedStore(t, defended)
		files := behind(t, NewServer("files", "read_file", "Lê."),
			"files", store)
		mail := behind(t, NewServer("mail", "Read-File", "[LAB] outra"),
			"mail", store)
		seen := append(toolNames(t, files), toolNames(t, mail)...)
		_, err := mail.CallTool(context.Background(),
			&mcp.CallToolParams{
				Name:      "Read-File",
				Arguments: map[string]any{"path": "notes.txt"},
			})
		t.Logf("defesa=%-5v o agente vê %v; Read-File: %v",
			defended, seen, err)
		shadowed := slices.Contains(seen, "Read-File")
		if shadowed == defended || (defended && err == nil) {
			t.Errorf("defesa=%v: sombreamento chegou=%v, err=%v",
				defended, shadowed, err)
		}
	}
}

// #endregion
