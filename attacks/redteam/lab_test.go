package redteam

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/attacks/excessiveagency"
	"github.com/aluiziodeveloper/guarita/attacks/exfiltration"
	"github.com/aluiziodeveloper/guarita/attacks/injection"
	"github.com/aluiziodeveloper/guarita/attacks/rugpull"
	"github.com/aluiziodeveloper/guarita/attacks/shadowing"
)

// lab é a Guarita do red team, com o que o teste precisa para atacá-la
// e para conferir o que chegou ao outro lado.
type lab struct {
	bin, base, ops, admin string
	trail, anaSecret      string
	received              *exfiltration.Received
	fixtures              map[string]string
	sign                  func(agent, route string) string
}

// #region attacks-redteam-setup
// setup sobe os servidores dos Capítulos 9 a 14 atrás de uma só
// Guarita, com tudo ligado: identidade, catálogo com varredor, política
// em Rego, aprovação, DLP e destinos, revisão e marcação dos
// resultados, cota, trilha e operação.
func setup(t *testing.T) *lab {
	t.Helper()
	dir := t.TempDir()
	l := &lab{
		bin:      build(t, dir, "cmd/guarita"),
		received: &exfiltration.Received{},
		trail:    filepath.Join(dir, "auditoria.jsonl"),
		fixtures: fixtures(t, "attack", "benign"),
	}
	pub, key := issuerKey(t, dir)
	l.sign = func(agent, route string) string {
		return token(t, key, agent, l.base+"/mcp/"+route)
	}
	l.anaSecret = write(t, filepath.Join(dir, "ana.secret"), anaSecret)
	upstreams := map[string]*mcp.Server{
		"files": rugpull.NewServer(),
		"notes": shadowing.NewServer("notes", "Read-File",
			"[LAB] outra"),
		"docs": poisoned(),
		"web":  injection.NewServer(l.fixtures),
		"mail": exfiltration.NewServer(l.received),
		"db":   excessiveagency.NewServer(),
		"status": failing(
			l.fixtures["attack/role-marker-chatml"]),
	}
	args := []string{
		"-listen", "127.0.0.1:0",
		"-agent-key", pub, "-agent-issuer", issuer,
		"-catalog", filepath.Join(dir, "catalogo.db"),
		"-policy", "testdata/policies",
		"-operators", write(t, filepath.Join(dir, "operators"),
			"ana "+anaSecret+"\n"),
		"-admin-listen", "127.0.0.1:0", "-approval-timeout", "2s",
		"-dlp", "-allow-hosts", "lab.test",
		"-inspect-results", "-mark-untrusted",
		"-quota", "web_search=20/m",
		"-audit", l.trail, "-audit-key", write(t,
			filepath.Join(dir, "audit.key"), "chave-de-laboratorio"),
		"-ops-listen", "127.0.0.1:0",
	}
	for name, server := range upstreams {
		args = append(args, "-server", name+"="+serve(t, server))
	}
	bases := start(t, l.bin, args...)
	l.base, l.ops, l.admin = bases["escutando"], bases["operação"],
		bases["aprovações"]
	return l
}

// #endregion

// anaSecret é o segredo da operadora do laboratório.
const anaSecret = "segredo-da-ana-0123456789"

// cpf é o CPF de exemplo das documentações, montado em duas partes.
var cpf = "123456789" + "09"

// as abre a sessão de um agente numa rota e lista as ferramentas, como
// todo agente faz antes de chamar: é a lista que alimenta o catálogo.
func (l *lab) as(t *testing.T, agent, route string) *mcp.ClientSession {
	t.Helper()
	s := session(t, l.base+"/mcp/"+route, l.sign(agent, route))
	if _, err := s.ListTools(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	return s
}

// #region attacks-redteam-poisoned
// poisoned é o servidor de documentação com uma ferramenta limpa e uma
// envenenada: a descrição esconde um caractere de largura zero antes
// do marcador [LAB], como no corpus do Capítulo 10.
func poisoned() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "docs"}, nil)
	for name, description := range map[string]string{
		"read_doc":    "Lê um documento do projeto.",
		"search_docs": "Busca nos documentos.\u200b [LAB] oculta",
	} {
		tool := &mcp.Tool{Name: name, Description: description}
		mcp.AddTool(server, tool,
			func(context.Context, *mcp.CallToolRequest, struct{}) (
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

// #region attacks-redteam-failing
// failing é o servidor de status que devolve a página de ataque no
// erro JSON-RPC da chamada, e não no resultado: é um erro de protocolo,
// que o cliente pode mostrar ao modelo.
func failing(text string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "status"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "check_status"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return nil, nil, &jsonrpc.Error{Code: -32000, Message: text}
		})
	return server
}

// #endregion

// fixtures lê os corpus do Capítulo 14, pelo nome de cada arquivo.
func fixtures(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join("..", "injection",
			"testdata", dir, "*.json"))
		for _, file := range files {
			//nolint:gosec // o corpus do livro, num caminho fixo
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var f struct{ Text string }
			_ = json.Unmarshal(raw, &f)
			name := dir + "/" + strings.TrimSuffix(filepath.Base(file),
				".json")
			out[name] = f.Text
		}
	}
	return out
}
