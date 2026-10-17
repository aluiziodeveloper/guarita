package mcphttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
)

// #region mcphttp-access-writer
// accessWriter guarda o status e o que o registro precisa saber da
// mensagem. Unwrap deixa ResponseController achar o Flush original:
// sem ele, os eventos do fluxo voltariam a ficar presos.
type accessWriter struct {
	http.ResponseWriter
	status     int
	mcpMethod  string
	name       string
	wroteFirst bool
}

func (w *accessWriter) WriteHeader(status int) {
	if !w.wroteFirst {
		w.status, w.wroteFirst = status, true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *accessWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// #endregion

// noteMessage anota o método e o nome da mensagem já aceita.
func noteMessage(w http.ResponseWriter, message mcpproxy.Message) {
	if access, ok := w.(*accessWriter); ok {
		mirrored := message.Mirrored()
		access.mcpMethod, access.name = message.Method, mirrored.Name
	}
}

// #region mcphttp-log-access
// logAccess escreve uma linha por pedido. Os campos são escolhidos um a
// um: nenhum cabeçalho nem corpo entra no registro.
func (h *Handler) logAccess(r *http.Request, w *accessWriter,
	start time.Time,
) {
	h.logger.LogAttrs(r.Context(), slog.LevelInfo, "mcp request",
		slog.String("http_method", r.Method),
		slog.String("mcp_method", w.mcpMethod),
		slog.String("name", w.name),
		slog.Int("status", w.status),
		slog.String("agent", h.agent(r.Context())),
		slog.Duration("duration", time.Since(start)))
}

// #endregion
