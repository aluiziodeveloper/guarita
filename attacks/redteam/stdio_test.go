package redteam

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stdioLab compila a Guarita e o servidor de arquivos do Capítulo 17 e
// prepara a raiz que ele pode ler, com um arquivo cujo nome tem o CPF.
func stdioLab(t *testing.T) (bin, files, root string) {
	t.Helper()
	dir := t.TempDir()
	bin = build(t, dir, "cmd/guarita")
	files = build(t, dir, "cmd/lab-files-mcp")
	root = filepath.Join(dir, "lab-files")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"relatorio.txt", cpf + ".txt"} {
		write(t, filepath.Join(root, name), "[LAB] "+name)
	}
	return bin, files, root
}

// stdioSession abre a sessão do agente com a Guarita no modo stdio.
func stdioSession(t *testing.T, bin string, args ...string) (
	*mcp.ClientSession, error,
) {
	t.Helper()
	cmd := exec.Command(bin, args...) //nolint:gosec // binário do teste
	s, err := mcp.NewClient(&mcp.Implementation{Name: "agente"}, nil).
		Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// #region attacks-redteam-stdio
// TestCostura_stdio repete a exfiltração e o laço contra a Guarita no
// modo stdio, com as mesmas opções da rota HTTP e a exigência de que
// a linha tenha dlp e quota. O CPF no caminho é negado, e o laço
// encontra a cota.
func TestCostura_stdio(t *testing.T) {
	bin, files, root := stdioLab(t)
	s, err := stdioSession(t, bin, "-allow", "read_file",
		"-dlp", "-quota", "read_file=3/m",
		"-require-stages", "dlp,quota",
		"--", files, "-root", root)
	if err != nil {
		t.Fatalf("a Guarita não subiu: %v", err)
	}
	leak := call(t, s, "read_file",
		map[string]any{"path": cpf + ".txt"})
	var passed int
	for range 5 {
		res := call(t, s, "read_file",
			map[string]any{"path": "relatorio.txt"})
		if !negada(res) {
			passed++
		}
	}
	t.Logf("o CPF no caminho: %s; laço de 5 leituras: %d chegaram",
		leak, passed)
	if !negada(leak) || passed > 3 {
		t.Fatal("o modo stdio deixou passar o que a linha barra")
	}
}

// #endregion

// #region attacks-redteam-stdio-refuses
// TestCostura_stdioRecusa sobe a Guarita no modo stdio com cada opção
// que só o modo HTTP aplica. Cada uma precisa impedir a partida, em
// vez de ser aceita e ignorada.
func TestCostura_stdioRecusa(t *testing.T) {
	bin, files, root := stdioLab(t)
	for _, extra := range [][]string{
		{"-inspect-results"},
		{"-mark-untrusted"},
		{"-pipeline", "injection"},
		{"-operators", "operadores"},
		{"-ops-listen", "127.0.0.1:0"},
		{"-trace-file", "spans.jsonl"},
	} {
		args := append([]string{"-allow", "read_file"}, extra...)
		args = append(args, "--", files, "-root", root)
		cmd := exec.Command(bin, args...) //nolint:gosec // do teste
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		refused := errors.As(err, &exit)
		t.Logf("%-24s recusada? %v: %s", strings.Join(extra, " "),
			refused, reason(string(out)))
		if !refused {
			t.Errorf("%v: a Guarita subiu e ignorou a opção", extra)
		}
	}
}

// #endregion

// reason é a mensagem de erro da Guarita, sem o carimbo de hora do log.
func reason(out string) string {
	for line := range strings.Lines(out) {
		if _, msg, ok := strings.Cut(line, " guarita: "); ok {
			return first(msg)
		}
	}
	return first(out)
}
