package mcphttp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

const agentToken, guaritaCred = "tok-do-usuario", "cred-da-guarita"

// loggedHandler é a Guarita com registro em logger e credencial própria.
func loggedHandler(t *testing.T, upstream string,
	logger *slog.Logger,
) http.Handler {
	t.Helper()
	agent := func(context.Context) string { return "agente-1" }
	target, _ := url.Parse(upstream + "/mcp")
	handler, _ := NewHandler(target, allowOnly(t, "read_file"),
		WithUpstreamCredential(secret.New(guaritaCred)),
		WithLogger(logger), WithAgent(agent))
	return handler
}

// #region mcphttp-logs-test
// Um pedido que chega e outro com o servidor fora do ar: o registro tem
// método, ferramenta, status e agente, e nenhum dos dois segredos.
func TestHandler_registroSemSegredos(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	up := httptest.NewServer(http.HandlerFunc(fakeMCP))
	t.Cleanup(up.Close)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	for _, server := range []string{up.URL, down.URL} {
		r := httptest.NewRequest("POST", "/mcp",
			strings.NewReader(toolCall(1, "read_file")))
		r.Header.Set("Authorization", "Bearer "+agentToken)
		loggedHandler(t, server, logger).ServeHTTP(
			httptest.NewRecorder(), r)
	}
	logger.Info("config", "credential", secret.New(guaritaCred))
	t.Logf("registro:\n%s", logs.String())
	for _, s := range []string{agentToken, guaritaCred} {
		if strings.Contains(logs.String(), s) {
			t.Errorf("o registro contém %q", s)
		}
	}
}

// #endregion
