package injection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/adapters/injection"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/adapters/untrusted"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// fixtures lê os arquivos de um diretório de testdata, pelo nome.
func fixtures(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("testdata", dir, "*.json"))
	out := map[string]string{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct{ Text string }
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		out[strings.TrimSuffix(filepath.Base(file), ".json")] = fixture.Text
	}
	return out
}

// behind põe o servidor atrás da Guarita, com a revisão dos resultados
// ligada ou não.
func behind(
	t *testing.T, texts map[string]string, defended bool,
) *mcp.ClientSession {
	t.Helper()
	server := NewServer(texts)
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(upstream.Close)
	policy, _ := policystatic.New(map[string]domain.Decision{
		"web_search": domain.DecisionAllow,
	})
	var options []app.Option
	if defended {
		options = append(options, app.WithServer("web"),
			app.WithResultInspectors(injection.New(
				injection.DefaultActions())),
			app.WithResultMarker(untrusted.New()))
	}
	uc := app.NewInterceptCall(policy, nil, options...)
	target, _ := url.Parse(upstream.URL + "/mcp")
	proxy, _ := mcphttp.NewHandler(target, mcpproxy.NewGate(uc),
		mcphttp.WithResults(uc))
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

// #region attacks-injection-test
// Cada fixture de ataque vem pela busca. Sem a revisão, o texto chega
// ao agente como veio. Com ela, os sinais fortes barram o resultado,
// e os fracos o deixam passar entre os delimitadores de conteúdo não
// confiável.
func TestInjection(t *testing.T) {
	texts := fixtures(t, "attack")
	for _, defended := range []bool{false, true} {
		session := behind(t, texts, defended)
		counts := map[string]int{}
		for _, name := range slices.Sorted(maps(texts)) {
			result, err := search(t, session, name)
			counts[outcome(result, err, texts[name])]++
		}
		t.Logf("revisão=%-5v %v", defended, counts)
		if (defended && counts["como veio"] > 0) ||
			(!defended && counts["como veio"] != len(texts)) {
			t.Errorf("revisão=%v: %v", defended, counts)
		}
	}
}

// #endregion

// outcome diz o que o agente recebeu: o texto como veio, o texto
// marcado ou o resultado barrado.
func outcome(result *mcp.CallToolResult, err error, sent string) string {
	switch got := text(result); {
	case err != nil || result.IsError:
		return "barrado"
	case strings.Contains(got, "[[conteúdo não confiável") &&
		strings.Contains(got, sent):
		return "marcado"
	case got == sent:
		return "como veio"
	}
	return "outro"
}

// search pede ao servidor a fixture de nome dado.
func search(
	t *testing.T, session *mcp.ClientSession, name string,
) (*mcp.CallToolResult, error) {
	return session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "web_search", Arguments: map[string]any{"q": name},
	})
}

// asResult escreve o texto como um resultado de ferramenta.
func asResult(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"content": []map[string]string{
		{"type": "text", "text": text},
	}})
	return raw
}

func text(result *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range result.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func maps(m map[string]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// #region attacks-injection-benign-test
// Os resultados legítimos do corpus benigno, pelo inspetor: os que ele
// barraria são os falsos positivos medidos.
func TestFalsosPositivos(t *testing.T) {
	inspector := injection.New(injection.DefaultActions())
	benign := fixtures(t, "benign")
	flagged, weak := 0, 0
	for _, name := range slices.Sorted(maps(benign)) {
		got, _ := inspector.InspectResult(t.Context(),
			domain.PolicyInput{}, asResult(benign[name]))
		if got.Decision != domain.DecisionAllow {
			flagged++
		} else if len(got.Findings) > 0 {
			weak++
		}
		t.Logf("%-28s %-5s %v", name, got.Decision, got.Findings)
	}
	t.Logf("de %d legítimos: %d barrados, %d com achado fraco",
		len(benign), flagged, weak)
	if flagged > 0 {
		t.Error("um resultado legítimo do corpus foi barrado")
	}
}

// #endregion
