package sandbox

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// needDocker pula o teste sem um Docker que responda.
func needDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("sem docker no PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("o Docker não responde")
	}
}

// shared cria um diretório que o usuário 65532 do contêiner consegue
// ler: o t.TempDir nasce com permissão só para o dono.
func shared(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{filepath.Dir(dir), dir} {
		if err := os.Chmod(d, 0o755); err != nil { //nolint:gosec // lab
			t.Fatal(err)
		}
	}
	return dir
}

// build compila um comando do módulo, estático, para o Linux.
func build(t *testing.T, dir, pkg, name string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out,
		"github.com/aluiziodeveloper/guarita/cmd/"+pkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, msg)
	}
	return out
}

// listen abre um ouvinte do teste em todas as interfaces, para que um
// contêiner com rede o alcance pelo host-gateway.
func listen(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "0.0.0.0:0") //nolint:gosec // lab
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}
