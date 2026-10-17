package rugpull

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sqlitestore"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// guarita põe a Guarita na frente do servidor de laboratório, com ou
// sem catálogo, e devolve uma sessão do cliente do SDK.
func guarita(t *testing.T, withCatalog bool) *mcp.ClientSession {
	t.Helper()
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return NewServer() }, nil))
	t.Cleanup(upstream.Close)
	policy, _ := policystatic.New(map[string]domain.Decision{
		"read_file": domain.DecisionAllow,
	})
	var useCase []app.Option
	var handler []mcphttp.Option
	if withCatalog {
		store, err := sqlitestore.Open(context.Background(),
			filepath.Join(t.TempDir(), "catalog.db"))
		if err != nil {
			t.Fatalf("Open() = %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		catalog := app.NewPinCatalog(store, upstream.URL)
		useCase = append(useCase, app.WithCatalog(catalog))
		handler = append(handler, mcphttp.WithCatalog(catalog))
	}
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil, useCase...))
	target, _ := url.Parse(upstream.URL + "/mcp")
	proxy, _ := mcphttp.NewHandler(target, gate, handler...)
	server := httptest.NewServer(proxy)
	t.Cleanup(server.Close)
	return connect(t, server.URL+"/mcp")
}

func connect(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "agent"}, nil)
	session, err := client.Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// schemaOf devolve o esquema de read_file como o agente o vê, ou "".
func schemaOf(t *testing.T, session *mcp.ClientSession) string {
	t.Helper()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() = %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "read_file" {
			schema, _ := json.Marshal(tool.InputSchema)
			return string(schema)
		}
	}
	return ""
}

func callReadFile(session *mcp.ClientSession) error {
	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "read_file", Arguments: map[string]any{"path": "notes.txt"},
	})
	return err
}

// #region attacks-rugpull-test
// Sem catálogo, o esquema alterado chega ao agente. Com catálogo,
// read_file some da lista e a chamada é negada.
func TestRugPull(t *testing.T) {
	for _, withCatalog := range []bool{false, true} {
		session := guarita(t, withCatalog)
		if !strings.Contains(schemaOf(t, session), `"path"`) {
			t.Fatalf("catálogo=%v: read_file não veio na 1ª lista",
				withCatalog)
		}
		if err := callReadFile(session); err != nil {
			t.Fatalf("catálogo=%v: 1ª chamada: %v", withCatalog, err)
		}
		changed := schemaOf(t, session)
		err := callReadFile(session)
		t.Logf("catálogo=%-5v marcador: %-5v 2ª chamada: %v",
			withCatalog, strings.Contains(changed, Marker), err)
		if attacked := strings.Contains(changed, Marker); attacked ==
			withCatalog {
			t.Errorf("catálogo=%v: o esquema alterado chegou=%v",
				withCatalog, attacked)
		}
		if withCatalog && (err == nil ||
			!strings.Contains(err.Error(), "definition changed")) {
			t.Errorf("com catálogo, a 2ª chamada passou: %v", err)
		}
	}
}

// #endregion
