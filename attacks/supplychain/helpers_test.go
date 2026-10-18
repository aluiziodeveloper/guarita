package supplychain

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

const module = "github.com/aluiziodeveloper/guarita/cmd/"

// build compila um comando do módulo em dir, com as etiquetas dadas.
func build(t *testing.T, dir, pkg, name string, tags ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	args := []string{"build", "-trimpath", "-o", out}
	for _, tag := range tags {
		args = append(args, "-tags", tag)
	}
	cmd := exec.Command("go", append(args, module+pkg)...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, msg)
	}
	return out
}

// digest é o digest do arquivo.
func digest(t *testing.T, path string) domain.Digest {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // binário do próprio teste
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	d, _ := domain.DigestOf(f)
	return d
}

// connect inicia o comando como servidor stdio e abre uma sessão. A
// saída de erro vai para stderr, onde o marcador aparece.
func connect(
	t *testing.T, stderr *bytes.Buffer, path string, args ...string,
) (*mcp.ClientSession, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.Command(path, args...)
	cmd.Stderr = stderr
	return mcp.NewClient(&mcp.Implementation{Name: "agent"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
}

// fingerprints são os digests das definições, como o catálogo do
// Capítulo 9 os fixa.
func fingerprints(t *testing.T, s *mcp.ClientSession) []string {
	t.Helper()
	tools, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, tool := range tools.Tools {
		raw, _ := json.Marshal(tool)
		f, err := domain.ToolDefinition{Name: tool.Name, Raw: raw}.
			Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(f))
	}
	return out
}
