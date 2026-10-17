package mcphttp

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// upstream imita um servidor MCP: o POST devolve um resultado em JSON e
// guarda o corpo recebido; o GET abre um fluxo de eventos.
type upstream struct {
	bodies  chan string
	release chan struct{}
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		u.bodies <- string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: primeiro\n\n")
		_ = http.NewResponseController(w).Flush()
		select { // espera o teste ler o primeiro evento
		case <-u.release:
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		_, _ = io.WriteString(w, "data: ultimo\n\n")
	}
}

func allowOnly(t *testing.T, tool string) *mcpproxy.Gate {
	t.Helper()
	policy, err := policystatic.New(map[string]domain.Decision{
		tool: domain.DecisionAllow,
	})
	if err != nil {
		t.Fatalf("policystatic.New() = %v", err)
	}
	return mcpproxy.NewGate(app.NewInterceptCall(policy, nil))
}

// newGuarita liga um Handler a um upstream de teste e devolve os dois.
func newGuarita(t *testing.T, next http.Handler) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(next)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL + "/mcp")
	handler, err := NewHandler(target, allowOnly(t, "read_file"))
	if err != nil {
		t.Fatalf("NewHandler() = %v", err)
	}
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)
	return guarita, guarita.URL + "/mcp"
}

func toolCall(id int, tool string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call",`+
		`"params":{"name":%q}}`, id, tool)
}

func post(t *testing.T, endpoint, body string) (*http.Response, string) {
	t.Helper()
	response, err := http.Post(endpoint, "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	got, _ := io.ReadAll(response.Body)
	return response, string(got)
}

// #region mcphttp-deny-test
func TestHandler_negaAntesDeRepassar(t *testing.T) {
	fake := &upstream{bodies: make(chan string, 2)}
	_, endpoint := newGuarita(t, fake)

	allowed := toolCall(1, "read_file")
	response, _ := post(t, endpoint, allowed)
	if response.StatusCode != http.StatusOK ||
		<-fake.bodies != allowed {
		t.Fatalf("a chamada permitida não chegou intacta ao servidor")
	}
	response, body := post(t, endpoint, toolCall(2, "delete_file"))
	if response.StatusCode != http.StatusOK ||
		!strings.Contains(body, `"id":2,"error"`) {
		t.Fatalf("negada: %d %s", response.StatusCode, body)
	}
	if len(fake.bodies) > 0 {
		t.Fatalf("a negada chegou ao servidor: %s", <-fake.bodies)
	}
}

// #endregion

// #region mcphttp-stream-test
// O primeiro evento precisa chegar enquanto o servidor ainda escreve. O
// prazo conta desde antes do pedido: http.Get só volta com os
// cabeçalhos, e um proxy que acumula atrasa os cabeçalhos também.
func TestHandler_repassaOFluxoSemAcumular(t *testing.T) {
	fake := &upstream{release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(fake.release) })
	t.Cleanup(release)
	_, endpoint := newGuarita(t, fake)

	first := make(chan string, 1)
	go func() {
		response, err := http.Get(endpoint)
		if err != nil {
			first <- err.Error()
			return
		}
		defer func() { _ = response.Body.Close() }()
		line, _ := bufio.NewReader(response.Body).ReadString('\n')
		first <- line
	}()
	select {
	case got := <-first:
		if got != "data: primeiro\n" {
			t.Fatalf("primeiro evento = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("o primeiro evento não chegou antes do fim do fluxo")
	}
}

// #endregion
