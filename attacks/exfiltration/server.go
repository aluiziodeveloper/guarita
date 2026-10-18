// Package exfiltration reproduz o vazamento do Capítulo 13: o agente
// manda, nos argumentos de uma ferramenta permitida, um CPF, uma chave
// ou um destino de fora.
package exfiltration

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Request são os argumentos de http_request.
type Request struct {
	URL  string `json:"url"`
	Body string `json:"body,omitempty"`
}

// Received guarda o que chegou ao servidor, para o teste conferir.
type Received struct {
	mu   sync.Mutex
	list []Request
}

// All devolve uma cópia do que chegou.
func (r *Received) All() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.list...)
}

// #region attacks-exfiltration-server
// NewServer anuncia http_request. Ela não faz pedido nenhum: guarda os
// argumentos que recebeu e devolve um marcador [LAB].
func NewServer(received *Received) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "web"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "http_request"},
		func(_ context.Context, _ *mcp.CallToolRequest, in Request) (
			*mcp.CallToolResult, any, error,
		) {
			received.mu.Lock()
			received.list = append(received.list, in)
			received.mu.Unlock()
			return &mcp.CallToolResult{Content: []mcp.Content{
				&mcp.TextContent{Text: "[LAB] recebido"},
			}}, nil, nil
		})
	return server
}

// #endregion
