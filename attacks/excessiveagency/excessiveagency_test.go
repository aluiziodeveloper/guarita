package excessiveagency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policyopa"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// behind põe o servidor atrás de um handler da Guarita com a política
// dada; o agente é sempre relatorios, e o servidor, db.
func behind(
	t *testing.T, policy app.PolicyEvaluator,
) *mcp.ClientSession {
	t.Helper()
	server := NewServer()
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(upstream.Close)
	agent := func(context.Context) string { return "relatorios" }
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil,
		app.WithServer("db"), app.WithAgent(agent)))
	target, _ := url.Parse(upstream.URL + "/mcp")
	proxy, _ := mcphttp.NewHandler(target, gate)
	guarita := httptest.NewServer(proxy)
	t.Cleanup(guarita.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
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

// marker devolve o texto do resultado, onde o servidor põe o [LAB].
func marker(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	text, _ := result.Content[0].(*mcp.TextContent)
	if text == nil {
		return ""
	}
	return text.Text
}

// globalList é a lista do Capítulo 4, feita para o agente de
// manutenção e aplicada a todos.
func globalList(t *testing.T) app.PolicyEvaluator {
	t.Helper()
	policy, err := policystatic.New(map[string]domain.Decision{
		"read_file":       domain.DecisionAllow,
		"delete_database": domain.DecisionAllow,
	})
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// #region attacks-excessiveagency-test
// O agente de relatórios chama delete_database. Com a lista global, a
// chamada chega ao servidor. Com a política versionada, que não
// concede a ferramenta a esse agente, ela é negada.
func TestExcessiveAgency(t *testing.T) {
	policies := os.DirFS("../../policies")
	versioned, err := policyopa.Load(t.Context(), policies)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	for _, c := range []struct {
		name   string
		policy app.PolicyEvaluator
	}{{"lista global", globalList(t)}, {"política", versioned}} {
		session := behind(t, c.policy)
		result, err := session.CallTool(t.Context(),
			&mcp.CallToolParams{Name: "delete_database"})
		reached := err == nil &&
			marker(result) == "[LAB] delete_database"
		t.Logf("%-12s delete_database chegou ao servidor? %v; %v",
			c.name, reached, err)
		if reached != (c.name == "lista global") {
			t.Errorf("%s: reached = %v", c.name, reached)
		}
	}
}

// #endregion
