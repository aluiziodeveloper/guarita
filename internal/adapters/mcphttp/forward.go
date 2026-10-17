package mcphttp

import (
	"net/http"
	"slices"
	"strings"
)

// #region mcphttp-allowed-headers
// forwarded são os únicos cabeçalhos do agente que chegam ao servidor:
// os do conteúdo e os do MCP, nas revisões 2025-11-25 e 2026-07-28.
var forwarded = []string{
	"Accept", "Content-Type",
	"MCP-Protocol-Version", "Mcp-Method", "Mcp-Name",
	"Mcp-Session-Id", "Last-Event-ID",
}

// paramPrefix marca os parâmetros de ferramenta espelhados em
// cabeçalhos; o servidor os confere contra o corpo.
const paramPrefix = "Mcp-Param-"

// allowedHeaders copia só o que está na lista. Credenciais, cookies e
// qualquer cabeçalho que ainda não exista ficam para trás.
func allowedHeaders(in http.Header) http.Header {
	out := make(http.Header)
	for _, name := range forwarded {
		if values := in.Values(name); len(values) > 0 {
			out[http.CanonicalHeaderKey(name)] = slices.Clone(values)
		}
	}
	for name, values := range in {
		if strings.HasPrefix(name, paramPrefix) {
			out[name] = slices.Clone(values)
		}
	}
	return out
}

// #endregion
