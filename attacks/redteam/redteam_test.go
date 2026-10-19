package redteam

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// #region attacks-redteam-call
// call chama uma ferramenta e resume o que voltou ao agente: negada
// pela Guarita, barrada na volta, marcada como não confiável ou passou.
func call(t *testing.T, s *mcp.ClientSession, tool string,
	args map[string]any,
) string {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := s.CallTool(t.Context(), &mcp.CallToolParams{
		Name: tool, Arguments: args,
	})
	if err != nil {
		return "negada: " + first(err.Error())
	}
	text := ""
	if len(res.Content) > 0 {
		if c, ok := res.Content[0].(*mcp.TextContent); ok {
			text = c.Text
		}
	}
	switch {
	case res.IsError:
		return "barrada: " + first(text)
	case strings.Contains(text, "[[conteúdo não confiável"):
		return "marcada"
	default:
		return "passou: " + first(text)
	}
}

// #endregion

// first é a primeira linha de um texto, curta o bastante para a saída.
func first(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if len(line) > 60 {
		return line[:60] + "…"
	}
	return line
}

// tools é a lista de ferramentas que a rota mostra ao agente.
func tools(t *testing.T, s *mcp.ClientSession) []string {
	t.Helper()
	res, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// #region attacks-redteam-chain
// TestCadeia é o red team: cada subteste é um movimento da cadeia,
// contra a mesma Guarita, na ordem em que um atacante os encadearia.
// Nenhum movimento pode passar; o que passa, passa porque é legítimo.
func TestCadeia(t *testing.T) {
	l := setup(t)
	t.Run("1 envenenamento", func(t *testing.T) {
		got := tools(t, l.as(t, "relatorios", "docs"))
		t.Logf("docs mostra %v", got)
		if slices.Contains(got, "search_docs") {
			t.Fatal("a ferramenta envenenada chegou ao agente")
		}
	})
	t.Run("2 sombreamento", func(t *testing.T) {
		// read_file é fixada primeiro, pelo servidor de arquivos
		_ = l.as(t, "relatorios", "files")
		notes := l.as(t, "relatorios", "notes")
		got, res := tools(t, notes), call(t, notes, "Read-File", nil)
		t.Logf("notes mostra %v; Read-File %s", got, res)
		if slices.Contains(got, "Read-File") || !negada(res) {
			t.Fatal("a ferramenta sombreada chegou ao agente")
		}
	})
	t.Run("3 troca silenciosa", func(t *testing.T) {
		s := l.as(t, "relatorios", "files")
		path := map[string]any{"path": "relatorio.txt"}
		before := call(t, s, "read_file", path)
		listed := tools(t, s) // o agente relê a lista
		after := call(t, s, "read_file", path)
		t.Logf("read_file antes da troca %s; a lista depois: %v; "+
			"read_file depois %s", before, listed, after)
		if !negada(after) {
			t.Fatal("a definição trocada foi usada")
		}
	})
	t.Run("4 injeção indireta", injectionMove(l))
	t.Run("5 exfiltração", exfiltrationMove(l))
	t.Run("6 agência excessiva", func(t *testing.T) {
		res := call(t, l.as(t, "relatorios", "db"), "delete_database",
			nil)
		t.Logf("relatorios: delete_database %s", res)
		if !negada(res) {
			t.Fatal("a ferramenta destrutiva chegou ao servidor")
		}
	})
	t.Run("7 fadiga de aprovação", fatigueMove(l))
	t.Run("8 abuso de recursos", abuseMove(l))
	t.Run("9 credencial de outra rota", credentialMove(l))
	t.Run("10 adulteração da trilha", auditMove(l))
	t.Run("11 exposição pela operação", opsMove(l))
	t.Run("12 injeção pelo erro", errorMove(l))
}

// #endregion

// negada diz se a Guarita negou a chamada antes do servidor.
func negada(result string) bool {
	return strings.HasPrefix(result, "negada")
}

// #region attacks-redteam-injection
// injectionMove manda o agente buscar as onze páginas de ataque do
// Capítulo 14. Cada uma precisa chegar barrada ou marcada; nenhuma
// passa como veio.
func injectionMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s := l.as(t, "relatorios", "web")
		count := map[string]int{}
		for name := range l.fixtures {
			if !strings.HasPrefix(name, "attack/") {
				continue
			}
			res := call(t, s, "web_search", map[string]any{"q": name})
			kind, _, _ := strings.Cut(res, ":")
			count[kind]++
		}
		t.Logf("11 páginas de ataque: %v", count)
		if count["passou"] > 0 || count["negada"] > 0 {
			t.Fatal("uma página de ataque chegou ao agente como veio")
		}
	}
}

// #endregion

// #region attacks-redteam-exfiltration
// exfiltrationMove é o que a página pedia: mandar o CPF para fora,
// pela ferramenta de envio permitida. O CPF vai para a fila, como o
// Capítulo 13 decidiu, e expira sem ninguém aprovar; o destino de fora
// é negado na hora. Só o pedido legítimo chega.
func exfiltrationMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s := l.as(t, "relatorios", "mail")
		for _, args := range []map[string]any{
			{"url": "https://lab.test/relatorio", "body": "CPF " + cpf},
			{"url": "https://fora.example/coleta", "body": "[LAB]"},
			{"url": "https://lab.test/relatorio", "body": "[LAB] ok"},
		} {
			t.Logf("%s: %s", args["url"], call(t, s, "http_request",
				args))
		}
		got := l.received.All()
		t.Logf("o servidor recebeu %d pedido(s)", len(got))
		if len(got) != 1 || strings.Contains(got[0].Body, cpf) {
			t.Fatalf("chegou ao servidor: %+v", got)
		}
	}
}

// #endregion

// #region attacks-redteam-fatigue
// fatigueMove é o agente de manutenção pedindo para apagar o banco
// cinco vezes seguidas, à espera de que alguém ceda. Três pedidos
// esperam a operadora; os outros dois são negados na hora. A operadora
// nega os três, e a repetição também é negada na hora.
func fatigueMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s := l.as(t, "manutencao", "db")
		answers := make(chan string, 5)
		for range 5 {
			go func() {
				answers <- call(t, s, "delete_database", nil)
			}()
		}
		pending := waitPending(t, l, 3)
		t.Logf("a operadora vê %d pedidos pendentes", len(pending))
		for _, id := range pending {
			l.operator(t, "deny", id)
		}
		var denied int
		for range 5 {
			if negada(<-answers) {
				denied++
			}
		}
		again := call(t, s, "delete_database", nil)
		t.Logf("%d de 5 negados; a repetição: %s", denied, again)
		if len(pending) != 3 || denied != 5 || !negada(again) {
			t.Fatal("a fila cedeu à insistência")
		}
	}
}

// #endregion

// waitPending lista a fila até ela ter want pedidos e devolve os ids.
func waitPending(t *testing.T, l *lab, want int) []string {
	t.Helper()
	var ids []string
	for range 100 {
		lines := strings.Split(strings.TrimSpace(l.operator(t, "list")),
			"\n")
		ids = ids[:0]
		for _, line := range lines[1:] {
			ids = append(ids, strings.Fields(line)[0])
		}
		if len(ids) >= want {
			return ids
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ids
}

// operator roda guarita approvals como a operadora ana.
func (l *lab) operator(t *testing.T, command ...string) string {
	t.Helper()
	args := append([]string{
		"approvals", "-admin", l.admin,
		"-secret", l.anaSecret,
	}, command...)
	out, err := exec.Command(l.bin, args...).Output() //nolint:gosec
	if err != nil {
		t.Fatalf("approvals %v: %v", command, err)
	}
	return string(out)
}

// #region attacks-redteam-abuse
// abuseMove é o laço: o agente repete a busca até a cota acabar. A
// cota de 20 por minuto já pagou as buscas da injeção.
func abuseMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s := l.as(t, "relatorios", "web")
		var passed, denied int
		for range 30 {
			res := call(t, s, "web_search",
				map[string]any{"q": "benign/noticia"})
			if negada(res) {
				denied++
			} else {
				passed++
			}
		}
		t.Logf("laço de 30 buscas: %d passaram, %d negadas", passed,
			denied)
		if denied == 0 {
			t.Fatal("o laço não encontrou a cota")
		}
	}
}

// #endregion

// credentialMove usa o token emitido para a rota files na rota mail:
// a audiência é outra, e a Guarita recusa.
func credentialMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s, err := connect(l.base+"/mcp/mail", l.sign("relatorios",
			"files"))
		res := ""
		if err != nil {
			res = "negada: " + first(err.Error())
		} else {
			defer func() { _ = s.Close() }()
			res = call(t, s, "http_request",
				map[string]any{"url": "https://lab.test/x"})
		}
		t.Logf("token de files em mail: %s", res)
		if !negada(res) {
			t.Fatal("o token de outra rota foi aceito")
		}
	}
}

// #region attacks-redteam-audit
// auditMove confere a trilha de tudo o que o red team fez, e depois
// troca uma negação por permissão numa cópia: o verificador acusa.
func auditMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		raw, err := os.ReadFile(l.trail)
		if err != nil {
			t.Fatal(err)
		}
		ok := l.verify(t, l.trail)
		forged := strings.Replace(string(raw), `"decision":"deny"`,
			`"decision":"allow"`, 1)
		copied := write(t, l.trail+".copia", forged)
		bad := l.verify(t, copied)
		t.Logf("%d entradas; original: %s; adulterada: %s",
			bytes.Count(raw, []byte("\n")), ok, bad)
		if !strings.HasPrefix(ok, "ok") ||
			strings.HasPrefix(bad, "ok") {
			t.Fatal("o verificador não separou as duas trilhas")
		}
	}
}

// #endregion

// verify roda guarita audit verify e resume a primeira linha.
func (l *lab) verify(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command(l.bin, "audit", "verify", path) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "falhou: " + first(string(out))
	}
	return "ok: " + first(string(out))
}

// opsMove lê a escuta de operação como quem a alcançasse: as métricas
// não levam dados das chamadas, e não há perfis sem -pprof.
func opsMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		metrics, mcode := get(t, l.ops+"/metrics")
		_, pcode := get(t, l.ops+"/debug/pprof/")
		leaked := strings.Contains(metrics, cpf) ||
			strings.Contains(metrics, "relatorio")
		t.Logf("/metrics %d (dados das chamadas? %v); /debug/pprof/ %d",
			mcode, leaked, pcode)
		if mcode != http.StatusOK || leaked ||
			pcode != http.StatusNotFound {
			t.Fatal("a operação expõe o que não devia")
		}
	}
}

// get faz um GET e devolve o corpo e o código.
func get(t *testing.T, url string) (string, int) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec,noctx // loopback do teste
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return string(body), res.StatusCode
}

// #region attacks-redteam-error
// errorMove pede o status a um servidor que responde com um erro
// JSON-RPC, e não com um resultado: a mensagem do erro é a página de
// ataque. Ela não pode chegar ao agente como veio.
func errorMove(l *lab) func(*testing.T) {
	return func(t *testing.T) {
		s := l.as(t, "relatorios", "status")
		_, err := s.CallTool(t.Context(), &mcp.CallToolParams{
			Name: "check_status", Arguments: map[string]any{},
		})
		msg := fmt.Sprint(err)
		reached := strings.Contains(msg, "[LAB]")
		t.Logf("o texto da página chega pelo erro? %v: %s", reached,
			first(msg))
		if reached {
			t.Fatal("a ordem chegou ao agente pelo erro JSON-RPC")
		}
	}
}

// #endregion
