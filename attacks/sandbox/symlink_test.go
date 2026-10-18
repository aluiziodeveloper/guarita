package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// files monta, ao lado da raiz de dados, o segredo do host, e dentro
// dela um relatório e dois symlinks para o segredo.
func files(t *testing.T) (data string) {
	t.Helper()
	dir := shared(t)
	data = filepath.Join(dir, "dados")
	secret := filepath.Join(dir, "segredo.txt")
	_ = os.Mkdir(data, 0o755) //nolint:gosec // o contêiner precisa ler
	_ = os.WriteFile(filepath.Join(data, "relatorio.txt"),
		[]byte("Relatório trimestral."), 0o644) //nolint:gosec // idem
	_ = os.WriteFile(secret, []byte("[LAB] segredo do host"), 0o600)
	_ = os.Symlink(secret, filepath.Join(data, "absoluto.txt"))
	_ = os.Symlink("../segredo.txt", filepath.Join(data, "relativo.txt"))
	return data
}

// guarita sobe a Guarita no modo stdio com os argumentos dados e abre
// uma sessão MCP com ela.
func guarita(t *testing.T, bin string, args ...string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.Command(filepath.Join(bin, "guarita"), args...)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "agent"},
		nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// read pede o arquivo e devolve o texto, ou o erro da ferramenta.
func read(t *testing.T, s *mcp.ClientSession, name string) string {
	t.Helper()
	result, err := s.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "read_file", Arguments: map[string]any{"path": name},
	})
	if err != nil {
		return "erro: " + err.Error()
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return "erro: " + text
	}
	return text
}

// #region attacks-sandbox-server-args
// serverArgs são os argumentos da Guarita para o servidor de arquivos
// no host ou na sandbox, com os binários e os dados montados só para
// leitura.
func serverArgs(bin, data, mode, where string) []string {
	if where == "host" {
		return []string{
			"-allow", "read_file", "--",
			filepath.Join(bin, "lab-files"),
			"-mode", mode, "-root", data,
		}
	}
	return []string{
		"-allow", "read_file", "-sandbox", "alpine:3.20",
		"-sandbox-mount", bin + ":/lab",
		"-sandbox-mount", data + ":/data",
		"--", "/lab/lab-files", "-mode", mode, "-root", "/data",
	}
}

// #endregion

// #region attacks-sandbox-symlink-test
// O mesmo servidor de arquivos, ingênuo ou com os.Root, no host ou na
// sandbox da Guarita. O segredo só sai do ingênuo no host: os.Root o
// recusa pelo código, e a sandbox, pelo que o contêiner não vê.
func TestFugaPorSymlink(t *testing.T) {
	needDocker(t)
	bin, data := shared(t), files(t)
	build(t, bin, "lab-files-mcp", "lab-files")
	build(t, bin, "guarita", "guarita")
	links := []string{"absoluto.txt", "relativo.txt"}
	for _, mode := range []string{"naive", "rooted"} {
		for _, where := range []string{"host", "sandbox"} {
			s := guarita(t, bin, serverArgs(bin, data, mode, where)...)
			for _, name := range links {
				got := read(t, s, name)
				t.Logf("%-6s %-7s %-12s %.60s", mode, where, name, got)
				leaked := strings.Contains(got, "[LAB]")
				if leaked != (mode == "naive" && where == "host") {
					t.Errorf("%s %s %s: %q", mode, where, name, got)
				}
			}
		}
	}
}

// #endregion
