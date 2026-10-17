package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const labIssuer = "https://auth.lab.test"

// labFiles grava a chave pública do emissor e a credencial da Guarita
// como o operador gravaria, e devolve a chave privada para emitir.
func labFiles(t *testing.T) (pub, cred string, key ed25519.PrivateKey) {
	t.Helper()
	dir := t.TempDir()
	public, private, _ := ed25519.GenerateKey(nil)
	der, _ := x509.MarshalPKIXPublicKey(public)
	pub, cred = filepath.Join(dir, "agent.pub"), filepath.Join(dir, "cred")
	block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := os.WriteFile(pub, block, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cred, []byte("cred-da-guarita\n"),
		0o600); err != nil {
		t.Fatal(err)
	}
	return pub, cred, private
}

func agentToken(t *testing.T, key ed25519.PrivateKey, aud string) string {
	t.Helper()
	return tokenFor(t, key, aud, "agente-1")
}

// tokenFor emite um token do laboratório para o agente dado.
func tokenFor(
	t *testing.T, key ed25519.PrivateKey, aud, agent string,
) string {
	t.Helper()
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodEdDSA,
		jwt.RegisteredClaims{
			Issuer: labIssuer, Subject: agent,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}).SignedString(key)
	return signed
}

// #region cmd-guarita-auth-test
func TestRun_identidadeECredencial(t *testing.T) {
	seen := make(chan string, 2)
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			seen <- r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{}}`)
		}))
	t.Cleanup(upstream.Close)
	pub, cred, key := labFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-allow", "read_file",
		"-listen", "127.0.0.1:0", "-upstream", upstream.URL+"/mcp",
		"-agent-key", pub, "-agent-issuer", labIssuer,
		"-upstream-credential", cred)

	status, challenge := authPost(t, endpoint, "")
	t.Logf("sem token: %d %s", status, challenge)
	if status != 401 ||
		!strings.Contains(challenge, "resource_metadata") {
		t.Fatalf("sem token: %d %q", status, challenge)
	}
	status, _ = authPost(t, endpoint, agentToken(t, key, endpoint))
	if got := <-seen; status != 200 || got != "Bearer cred-da-guarita" {
		t.Fatalf("com token: %d; o servidor recebeu %q", status, got)
	}
}

// #endregion

func authPost(t *testing.T, endpoint, token string) (int, string) {
	t.Helper()
	r, _ := http.NewRequest("POST", endpoint,
		strings.NewReader(toolCall(1, "read_file")))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = response.Body.Close()
	return response.StatusCode, response.Header.Get("WWW-Authenticate")
}

func TestRun_metadadosDoRecurso(t *testing.T) {
	pub, _, _ := labFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-listen", "127.0.0.1:0",
		"-upstream", "http://127.0.0.1:1/mcp",
		"-agent-key", pub, "-agent-issuer", labIssuer)
	metadata := strings.TrimSuffix(endpoint, "/mcp") +
		"/.well-known/oauth-protected-resource"
	response, err := http.Get(metadata)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	t.Logf("%s", body)
	if !bytes.Contains(body, []byte(`"resource":"`+endpoint+`"`)) ||
		!bytes.Contains(body, []byte(labIssuer)) {
		t.Fatalf("metadados: %s", body)
	}
}

func TestRun_foraDoLoopbackExigeIdentidade(t *testing.T) {
	err := run(context.Background(), []string{
		"-listen", "127.0.0.1:0", "-host", "guarita.lab.test:8080",
		"-upstream", "http://127.0.0.1:1/mcp",
	}, io.NopCloser(strings.NewReader("")),
		nopWriteCloser{&bytes.Buffer{}}, io.Discard)
	if !errors.Is(err, errAuthRequired) {
		t.Fatalf("run() = %v; want errAuthRequired", err)
	}
}
