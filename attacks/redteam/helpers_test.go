package redteam

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const issuer = "https://auth.lab.test"

// build compila um programa do módulo, como a release o compila, e
// devolve o caminho do binário.
func build(t *testing.T, dir, pkg string) string {
	t.Helper()
	out := filepath.Join(dir, filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-trimpath", "-o", out,
		"github.com/aluiziodeveloper/guarita/"+pkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, msg)
	}
	return out
}

// issuerKey gera a chave do emissor do laboratório e grava a pública.
func issuerKey(t *testing.T, dir string) (string, ed25519.PrivateKey) {
	t.Helper()
	public, private, _ := ed25519.GenerateKey(nil)
	der, _ := x509.MarshalPKIXPublicKey(public)
	path := filepath.Join(dir, "agent.pub")
	block := pem.EncodeToMemory(&pem.Block{
		Type: "PUBLIC KEY", Bytes: der,
	})
	//nolint:gosec // a chave pública pode ser lida por todos
	if err := os.WriteFile(path, block, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, private
}

// token emite o token de um agente para uma rota.
func token(
	t *testing.T, key ed25519.PrivateKey, agent, aud string,
) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA,
		jwt.RegisteredClaims{
			Issuer: issuer, Subject: agent,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// serve põe um servidor MCP de laboratório numa porta do teste.
func serve(t *testing.T, server *mcp.Server) string {
	t.Helper()
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(upstream.Close)
	return upstream.URL
}

// start roda a Guarita e devolve o que ela anuncia na saída de erro ao
// subir: "escutando", "operação" e "aprovações", cada um com a base.
func start(t *testing.T, bin string, args ...string) map[string]string {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, bin, args...)
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	bases := map[string]string{}
	lines := bufio.NewScanner(stderr)
	for lines.Scan() {
		rest, ok := strings.CutPrefix(lines.Text(), "guarita: ")
		what, base, found := strings.Cut(rest, " em ")
		if !ok || !found {
			continue
		}
		base, _, _ = strings.Cut(strings.TrimSuffix(base, "/"), "/mcp")
		bases[what] = base
		if what == "escutando" {
			go func() { _, _ = io.Copy(io.Discard, stderr) }()
			return bases
		}
	}
	t.Fatal("a Guarita não anunciou a escuta")
	return nil
}

// bearer põe o token em todo pedido.
type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}

// connect abre a sessão do agente numa rota da Guarita.
func connect(endpoint, tok string) (*mcp.ClientSession, error) {
	client := &http.Client{
		Transport: bearer{tok, http.DefaultTransport},
	}
	return mcp.NewClient(&mcp.Implementation{Name: "agente"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{
			Endpoint: endpoint, HTTPClient: client,
		}, nil)
}

// session é connect para o caminho feliz: falhar aqui encerra o teste.
func session(t *testing.T, endpoint, tok string) *mcp.ClientSession {
	t.Helper()
	s, err := connect(endpoint, tok)
	if err != nil {
		t.Fatalf("%s: %v", endpoint, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// write grava um arquivo do laboratório com permissão 0600.
func write(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
