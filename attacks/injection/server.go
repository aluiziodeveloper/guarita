// Package injection reproduz a injeção indireta do Capítulo 14: o
// resultado de uma busca permitida traz texto escrito para o modelo.
// As fixtures de testdata/attack têm só marcadores [LAB]; as de
// testdata/benign são resultados legítimos que se parecem com elas.
package injection

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Query é o argumento de web_search: o nome da fixture.
type Query struct {
	Q string `json:"q"`
}

// #region attacks-injection-server
// NewServer anuncia web_search, que devolve o texto da fixture pedida.
// Não busca nada: o resultado é o que o teste escolheu.
func NewServer(fixtures map[string]string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "web"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "web_search"},
		func(_ context.Context, _ *mcp.CallToolRequest, in Query) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{Content: []mcp.Content{
				&mcp.TextContent{Text: fixtures[in.Q]},
			}}, nil, nil
		})
	return server
}

// #endregion
