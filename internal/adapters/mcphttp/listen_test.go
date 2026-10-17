package mcphttp

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// slowStream responde com um fluxo cujo primeiro evento demora pause.
func slowStream(pause time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_ = http.NewResponseController(w).Flush()
		select {
		case <-time.After(pause):
			_, _ = io.WriteString(w, "data: lista mudou\n\n")
		case <-r.Context().Done():
		}
	}
}

// #region mcphttp-listen-test
// Com prazo de 100 ms, o fluxo de subscriptions/listen entrega o evento
// de 300 ms; o de um tools/call com a mesma demora é cortado no prazo.
func TestHandler_listenSemPrazoDeChamada(t *testing.T) {
	upstream := httptest.NewServer(slowStream(300 * time.Millisecond))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "read_file"),
		WithCallTimeout(100*time.Millisecond))
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)

	listen := `{"jsonrpc":"2.0","id":1,"method":"subscriptions/listen"}`
	got := firstLine(t, guarita.URL, listen)
	if got != "data: lista mudou\n" {
		t.Fatalf("subscriptions/listen: %q", got)
	}
	start := time.Now()
	got = firstLine(t, guarita.URL, toolCall(2, "read_file"))
	elapsed := time.Since(start)
	t.Logf("tools/call: %q depois de %v", got,
		elapsed.Round(10*time.Millisecond))
	if got != "" || elapsed >= 300*time.Millisecond {
		t.Fatalf("tools/call não foi cortado no prazo: %q", got)
	}
}

// #endregion

func firstLine(t *testing.T, endpoint, body string) string {
	t.Helper()
	response, err := http.Post(endpoint, "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	reader := bufio.NewReader(response.Body)
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimSpace(line) != "" || err != nil {
			return line
		}
	}
}
