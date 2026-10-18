// Command lab-files-mcp é o servidor MCP de arquivos do laboratório do
// Capítulo 17. Ele serve read_file pela entrada e saída padrão, sobre
// a raiz dada, no modo ingênuo ou com os.Root. Existe só para o
// laboratório: o conteúdo que ele lê é o que o teste põe na raiz.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type readFileArgs struct {
	Path string `json:"path" jsonschema:"file path inside the root"`
}

func main() {
	dir := flag.String("root", "/data", "directory the server may read")
	mode := flag.String("mode", "rooted", "naive or rooted")
	flag.Parse()
	read := naive(*dir)
	if *mode == "rooted" {
		r, closeRoot, err := rooted(*dir)
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = closeRoot() }()
		read = r
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name: "lab-files", Version: "v0.1.0",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name: "read_file", Description: "Reads a text file.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readFileArgs) (
		*mcp.CallToolResult, any, error,
	) {
		data, err := read(in.Path)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{
				&mcp.TextContent{Text: err.Error()},
			}}, nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		}}, nil, nil
	})
	if err := server.Run(context.Background(),
		&mcp.StdioTransport{}); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
