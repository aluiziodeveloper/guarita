package mcpref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// wire guarda as linhas que o LoggingTransport escreve, com trava,
// porque as leituras chegam pela goroutine do transporte.
type wire struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *wire) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// reset descarta o que passou até aqui, como o aperto de mão.
func (w *wire) reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Reset()
}

// flush imprime o que passou desde a última chamada, cada mensagem com
// o sentido visto pelo cliente e o JSON indentado.
func (w *wire) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for line := range strings.SplitSeq(strings.TrimSpace(w.buf.String()), "\n") {
		dir, msg, _ := strings.Cut(line, ": ")
		var out bytes.Buffer
		if err := json.Indent(&out, []byte(msg), "", "  "); err != nil {
			out.WriteString(msg)
		}
		arrow := "cliente → servidor"
		if dir == "read" {
			arrow = "servidor → cliente"
		}
		fmt.Printf("%s\n%s\n", arrow, out.String())
	}
	w.buf.Reset()
}

type readArgs struct {
	Path string `json:"path" jsonschema:"o caminho do arquivo"`
}

// labServer anuncia read_file, que lê só relatorio.txt e devolve um
// erro de ferramenta para qualquer outro caminho.
func labServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "lab-files",
		Version: "v0.1.0",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_file",
		Description: "Lê um arquivo do diretório do laboratório.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readArgs) (
		*mcp.CallToolResult, any, error,
	) {
		if in.Path != "relatorio.txt" {
			return nil, nil, errors.New("file not found: " + in.Path)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: "[LAB] vendas estáveis"},
		}}, nil, nil
	})
	return server
}

// connect liga o cliente ao servidor do laboratório na revisão pedida e
// devolve a sessão e o registro do fio.
func connect(ctx context.Context, version string) (*mcp.ClientSession,
	*wire, func(),
) {
	st, ct := mcp.NewInMemoryTransports()
	ss, err := labServer().Connect(ctx, st, nil)
	if err != nil {
		log.Fatal(err)
	}
	w := &wire{}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "agente-lab",
		Version: "v0.1.0",
	}, nil)
	cs, err := client.Connect(ctx, &mcp.LoggingTransport{
		Transport: ct,
		Writer:    w,
	}, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		log.Fatal(err)
	}
	return cs, w, func() { _ = cs.Close(); _ = ss.Close() }
}

// #region sessao-2025
// Na revisão 2025-11-25, a sessão começa pelo aperto de mão:
// initialize, a resposta com as capacidades e a notificação de
// inicializado, que não tem id.
func Example_aperto2025() {
	ctx := context.Background()
	_, w, done := connect(ctx, "2025-11-25")
	defer done()
	w.flush()
	// Output:
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 1,
	//   "method": "initialize",
	//   "params": {
	//     "clientInfo": {
	//       "name": "agente-lab",
	//       "version": "v0.1.0"
	//     },
	//     "protocolVersion": "2025-11-25",
	//     "capabilities": {
	//       "roots": {
	//         "listChanged": true
	//       }
	//     }
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 1,
	//   "result": {
	//     "capabilities": {
	//       "logging": {},
	//       "tools": {
	//         "listChanged": true
	//       }
	//     },
	//     "protocolVersion": "2025-11-25",
	//     "serverInfo": {
	//       "name": "lab-files",
	//       "version": "v0.1.0"
	//     }
	//   }
	// }
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "method": "notifications/initialized",
	//   "params": {}
	// }
}

// #endregion

// #region sessao-lista-chamada
// tools/list devolve a definição de cada ferramenta, com o esquema de
// entrada; tools/call leva o nome e os argumentos, e o resultado volta
// em blocos de conteúdo.
func Example_listaEChamada() {
	ctx := context.Background()
	cs, w, done := connect(ctx, "2025-11-25")
	defer done()
	w.reset()
	if _, err := cs.ListTools(ctx, nil); err != nil {
		log.Fatal(err)
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "relatorio.txt"},
	}); err != nil {
		log.Fatal(err)
	}
	w.flush()
	// Output:
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "method": "tools/list"
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "result": {
	//     "ttlMs": 0,
	//     "cacheScope": "public",
	//     "tools": [
	//       {
	//         "description": "Lê um arquivo do diretório do laboratório.",
	//         "inputSchema": {
	//           "type": "object",
	//           "properties": {
	//             "path": {
	//               "type": "string",
	//               "description": "o caminho do arquivo"
	//             }
	//           },
	//           "required": [
	//             "path"
	//           ],
	//           "additionalProperties": false
	//         },
	//         "name": "read_file"
	//       }
	//     ]
	//   }
	// }
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 3,
	//   "method": "tools/call",
	//   "params": {
	//     "name": "read_file",
	//     "arguments": {
	//       "path": "relatorio.txt"
	//     }
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 3,
	//   "result": {
	//     "content": [
	//       {
	//         "type": "text",
	//         "text": "[LAB] vendas estáveis"
	//       }
	//     ]
	//   }
	// }
}

// #endregion

// #region sessao-erros
// Os dois jeitos de uma chamada dar errado: a ferramenta que falha
// devolve um resultado com isError, que o modelo pode ler; a
// ferramenta que não existe recebe um erro JSON-RPC, com code e
// message no lugar de result.
func Example_doisErros() {
	ctx := context.Background()
	cs, w, done := connect(ctx, "2025-11-25")
	defer done()
	w.reset()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "segredo.txt"},
	})
	fmt.Println("isError:", res.IsError, "erro Go:", err)
	_, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "delete_file"})
	fmt.Println("erro Go:", err)
	w.flush()
	// Output:
	// isError: true erro Go: <nil>
	// erro Go: calling "tools/call": unknown tool "delete_file"
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "method": "tools/call",
	//   "params": {
	//     "name": "read_file",
	//     "arguments": {
	//       "path": "segredo.txt"
	//     }
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "result": {
	//     "content": [
	//       {
	//         "type": "text",
	//         "text": "file not found: segredo.txt"
	//       }
	//     ],
	//     "isError": true
	//   }
	// }
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 3,
	//   "method": "tools/call",
	//   "params": {
	//     "name": "delete_file",
	//     "arguments": {}
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 3,
	//   "error": {
	//     "code": -32602,
	//     "message": "unknown tool \"delete_file\""
	//   }
	// }
}

// #endregion

// #region sessao-2026
// Na revisão 2026-07-28 não há initialize: o cliente descobre o
// servidor com server/discover, e cada pedido leva em _meta a versão e
// as capacidades do cliente.
func Example_semEstado2026() {
	ctx := context.Background()
	cs, w, done := connect(ctx, "2026-07-28")
	defer done()
	fmt.Println("revisão:", cs.InitializeResult().ProtocolVersion)
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "relatorio.txt"},
	}); err != nil {
		log.Fatal(err)
	}
	w.flush()
	// Output:
	// revisão: 2026-07-28
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 1,
	//   "method": "server/discover",
	//   "params": {
	//     "_meta": {
	//       "io.modelcontextprotocol/clientCapabilities": {
	//         "roots": {
	//           "listChanged": true
	//         }
	//       },
	//       "io.modelcontextprotocol/clientInfo": {
	//         "name": "agente-lab",
	//         "version": "v0.1.0"
	//       },
	//       "io.modelcontextprotocol/protocolVersion": "2026-07-28"
	//     }
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 1,
	//   "result": {
	//     "resultType": "complete",
	//     "_meta": {
	//       "io.modelcontextprotocol/serverInfo": {
	//         "name": "lab-files",
	//         "version": "v0.1.0"
	//       }
	//     },
	//     "ttlMs": 0,
	//     "cacheScope": "public",
	//     "supportedVersions": [
	//       "2026-07-28",
	//       "2025-11-25",
	//       "2025-06-18",
	//       "2025-03-26",
	//       "2024-11-05"
	//     ],
	//     "capabilities": {
	//       "logging": {},
	//       "tools": {
	//         "listChanged": true
	//       }
	//     }
	//   }
	// }
	// cliente → servidor
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "method": "tools/call",
	//   "params": {
	//     "_meta": {
	//       "io.modelcontextprotocol/clientCapabilities": {
	//         "roots": {
	//           "listChanged": true
	//         }
	//       },
	//       "io.modelcontextprotocol/clientInfo": {
	//         "name": "agente-lab",
	//         "version": "v0.1.0"
	//       },
	//       "io.modelcontextprotocol/protocolVersion": "2026-07-28"
	//     },
	//     "name": "read_file",
	//     "arguments": {
	//       "path": "relatorio.txt"
	//     }
	//   }
	// }
	// servidor → cliente
	// {
	//   "jsonrpc": "2.0",
	//   "id": 2,
	//   "result": {
	//     "_meta": {
	//       "io.modelcontextprotocol/serverInfo": {
	//         "name": "lab-files",
	//         "version": "v0.1.0"
	//       }
	//     },
	//     "content": [
	//       {
	//         "type": "text",
	//         "text": "[LAB] vendas estáveis"
	//       }
	//     ],
	//     "resultType": "complete"
	//   }
	// }
}

// #endregion
