package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
)

type run struct {
	name string
	cmd  *exec.Cmd
}

// #region attacks-sandbox-probe-runs
// probeRuns monta a sonda nos três lugares. O contêiner padrão tem só
// o que o Docker dá sem opções, mais o host-gateway para alcançar o
// ouvinte; o perfil da Guarita é o de Command, sem nada a mais.
func probeRuns(t *testing.T, bin, secret, port string) []run {
	args := func(host string) []string {
		return []string{
			"/probe/sonda", "-connect", host + ":" + port,
			"-secret", secret,
		}
	}
	inside := args("host.docker.internal")
	profile := sandbox.Default("alpine:3.20")
	profile.Mounts = []sandbox.Mount{{Source: bin, Target: "/probe"}}
	guarded, name, _ := profile.Command(t.Context(), inside)
	t.Cleanup(func() { _ = sandbox.Remove(name) })
	plain := append([]string{
		"run", "--rm", "--add-host",
		"host.docker.internal:host-gateway", "-v", bin + ":/probe:ro",
		"alpine:3.20",
	}, inside...)
	return []run{
		{"host", exec.Command(filepath.Join(bin, "sonda"),
			args("127.0.0.1")[1:]...)},
		{"contêiner padrão", exec.Command("docker", plain...)},
		{"perfil da Guarita", guarded},
	}
}

// #endregion

// #region attacks-sandbox-probe-test
// A sonda em três lugares: no host, num contêiner com as opções padrão
// do Docker e no perfil da Guarita. Só o perfil nega as cinco
// tentativas.
func TestSonda(t *testing.T) {
	needDocker(t)
	bin := shared(t)
	build(t, bin, "lab-sandbox-probe", "sonda")
	secret := filepath.Join(t.TempDir(), "segredo.txt")
	_ = os.WriteFile(secret, []byte("[LAB] segredo do host"), 0o600)
	for _, r := range probeRuns(t, bin, secret, listen(t)) {
		out, _ := r.cmd.CombinedOutput()
		t.Logf("%s:\n%s", r.name, out)
		if r.name == "perfil da Guarita" &&
			strings.Count(string(out), "negado") != 5 {
			t.Error("o perfil deixou passar alguma tentativa")
		}
	}
}

// #endregion
