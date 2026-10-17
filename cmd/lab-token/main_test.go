package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/identity"
)

// O token emitido pela ferramenta passa no verificador da Guarita, e a
// chave privada nasce legível só pelo dono.
func TestRun_geraChavesEEmiteToken(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"-genkey", "-out", dir}, nil); err != nil {
		t.Fatalf("-genkey: %v", err)
	}
	info, _ := os.Stat(filepath.Join(dir, "agent.key"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("agent.key tem modo %v", info.Mode().Perm())
	}
	var out bytes.Buffer
	const iss, aud = "https://auth.lab.test", "http://127.0.0.1:8080/mcp"
	err := run([]string{
		"-key", filepath.Join(dir, "agent.key"),
		"-iss", iss, "-aud", aud, "-sub", "agente-1",
	}, &out)
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}
	pub, _ := os.ReadFile(filepath.Join(dir, "agent.pub"))
	key, err := identity.ParsePublicKey(pub)
	if err != nil {
		t.Fatalf("ParsePublicKey() = %v", err)
	}
	v, _ := identity.NewVerifier(key, iss, aud)
	info2, err := v.Verify(t.Context(), strings.TrimSpace(out.String()),
		nil)
	if err != nil || info2.UserID != "agente-1" {
		t.Fatalf("Verify() = %v, %v", info2, err)
	}
}
