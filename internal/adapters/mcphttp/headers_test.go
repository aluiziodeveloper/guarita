package mcphttp

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// call2026 é um tools/call da revisão 2026-07-28, com a versão em _meta.
func call2026(tool string) string {
	return `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{` +
		`"name":"` + tool + `","arguments":{},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
}

func mirrored(method, name string) http.Header {
	header := http.Header{
		"Mcp-Protocol-Version": {"2026-07-28"},
		"Mcp-Method":           {method},
	}
	if name != "" {
		header["Mcp-Name"] = []string{name}
	}
	return header
}

// #region mcphttp-mirrored-test
// A decisão segue o corpo: delete_file é negada mesmo com Mcp-Name
// read_file. Um cabeçalho diferente do corpo permitido é recusado.
func TestHandler_cabecalhosEspelhados(t *testing.T) {
	endpoint := newGuarded(t, http.HandlerFunc(fakeMCP))
	read, del := call2026("read_file"), call2026("delete_file")
	same := mirrored("tools/call", "read_file")
	encoded := mirrored("tools/call", "=?base64?"+
		base64.StdEncoding.EncodeToString([]byte("read_file"))+"?=")
	twice := mirrored("tools/call", "read_file")
	twice.Add("Mcp-Name", "delete_file")
	other := mirrored("tools/call", "list_dir")
	noName := mirrored("tools/call", "")
	method := mirrored("tools/list", "read_file")
	old := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	for _, c := range []contract{
		{"iguais", "POST", read, same, 200, `"result"`},
		{"base64", "POST", read, encoded, 200, `"result"`},
		{"corpo negado", "POST", del, same, 200, `-32001`},
		{"nome divergente", "POST", read, other, 400, `-32020`},
		{"nome repetido", "POST", read, twice, 400, `-32020`},
		{"sem Mcp-Name", "POST", read, noName, 400, `-32020`},
		{"método divergente", "POST", read, method, 400, `-32020`},
		{"revisão antiga", "POST", old, nil, 200, `"result"`},
	} {
		t.Run(c.name, func(t *testing.T) { c.check(t, endpoint) })
	}
}

// #endregion

func TestCheckMirrored_versaoDivergente(t *testing.T) {
	body := strings.Replace(call2026("read_file"), "2026-07-28",
		"2025-11-25", 1)
	endpoint := newGuarded(t, http.HandlerFunc(fakeMCP))
	contract{
		"versão", "POST", body, mirrored("tools/call", "read_file"),
		400, `MCP-Protocol-Version`,
	}.check(t, endpoint)
}
