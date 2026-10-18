package mcphttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// agentTrace é o traceparent do agente: versão, trace, span e flags.
const agentTrace = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

// traceServer guarda o traceparent que chegou e responde vazio.
func traceServer(t *testing.T, received *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			*received = r.Header.Get("traceparent")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}))
	t.Cleanup(server.Close)
	return server
}

// #region mcphttp-trace-test
// O agente manda um traceparent; o servidor recebe o mesmo trace, com
// o span da Guarita como pai, e não o do agente. O span da Guarita,
// registrado pelo SDK, é filho do span do agente.
func TestHandler_trace(t *testing.T) {
	var received string
	upstream := traceServer(t, &received)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder))
	target, _ := url.Parse(upstream.URL + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "read_file"),
		WithTracer(provider.Tracer("guarita")))
	r := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(toolCall(1, "read_file")))
	r.Header.Set("traceparent", agentTrace)
	handler.ServeHTTP(httptest.NewRecorder(), r)
	span := recorder.Ended()[0]
	t.Logf("agente:   %s", agentTrace)
	t.Logf("servidor: %s", received)
	t.Logf("span da Guarita %s, pai %s", span.SpanContext().SpanID(),
		span.Parent().SpanID())
	if !strings.HasPrefix(received, agentTrace[:36]) ||
		received == agentTrace ||
		span.Parent().SpanID().String() != agentTrace[36:52] {
		t.Error("o trace não foi carregado adiante")
	}
}

// #endregion
