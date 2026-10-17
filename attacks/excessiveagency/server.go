// Package excessiveagency reproduz a agência excessiva do Capítulo 11:
// um servidor de laboratório oferece uma ferramenta destrutiva, e a
// lista global de permitidas a entrega a um agente que só precisava
// ler.
package excessiveagency

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type args struct{}

// #region attacks-excessiveagency-server
// NewServer anuncia read_file e delete_database. Nenhuma das duas toca
// em nada: cada uma devolve um marcador [LAB] com o próprio nome.
func NewServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "db"}, nil)
	for _, name := range []string{"read_file", "delete_database"} {
		mcp.AddTool(server, &mcp.Tool{Name: name},
			func(context.Context, *mcp.CallToolRequest, args) (
				*mcp.CallToolResult, any, error,
			) {
				return &mcp.CallToolResult{Content: []mcp.Content{
					&mcp.TextContent{Text: "[LAB] " + name},
				}}, nil, nil
			})
	}
	return server
}

// #endregion
