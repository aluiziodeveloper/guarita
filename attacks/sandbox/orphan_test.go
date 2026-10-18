package sandbox

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
)

// running diz se o contêiner com o nome dado ainda está de pé.
func running(name string) bool {
	out, _ := exec.Command("docker", "ps", "-q", "--filter",
		"name=^"+name+"$").Output()
	return strings.TrimSpace(string(out)) != ""
}

// gone espera até cinco segundos o contêiner sumir.
func gone(name string) bool {
	for range 25 {
		if !running(name) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// start inicia o contêiner e espera ele ficar de pé.
func start(t *testing.T, cmd *exec.Cmd, name string) {
	t.Helper()
	t.Cleanup(func() { _ = sandbox.Remove(name) })
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for !running(name) {
		time.Sleep(200 * time.Millisecond)
	}
}

// #region attacks-sandbox-orphan-test
// Cancelar o contexto manda SIGTERM ao cliente do Docker, e o
// contêiner some. O controle negativo mata o cliente com SIGKILL: o
// contêiner fica de pé até Remove.
func TestSemOrfao(t *testing.T) {
	needDocker(t)
	for _, kill := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		cmd, name, _ := sandbox.Default("alpine:3.20").Command(ctx,
			[]string{"sleep", "600"})
		start(t, cmd, name)
		if kill {
			_ = cmd.Process.Signal(syscall.SIGKILL)
		}
		cancel()
		_ = cmd.Wait()
		orphan := !gone(name)
		t.Logf("SIGKILL=%-5v contêiner órfão=%v", kill, orphan)
		_ = sandbox.Remove(name)
		t.Logf("SIGKILL=%-5v depois de Remove, de pé=%v", kill,
			running(name))
		if orphan != kill || running(name) {
			t.Errorf("SIGKILL=%v: comportamento mudou", kill)
		}
	}
}

// #endregion
