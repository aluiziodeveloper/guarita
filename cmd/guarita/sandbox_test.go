package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
)

// fakeHome aponta HOME para um diretório temporário com .ssh, para o
// teste valer em qualquer máquina.
func fakeHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.Mkdir(filepath.Join(home, ".ssh"), 0o700)
	return home
}

// sandboxProfile lê as opções e monta o perfil, como na partida.
func sandboxProfile(args ...string) (*sandbox.Profile, error) {
	cfg, err := parseConfig(append([]string{"-allow", "x"}, args...),
		&strings.Builder{})
	if err != nil {
		return nil, err
	}
	return cfg.sandbox.profile()
}

// #region cmd-guarita-sandbox-config-test
// A sandbox não sobe com a montagem mal escrita, no modo HTTP nem com
// a home montada; a home é falsa, com .ssh.
func TestSandbox_recusas(t *testing.T) {
	home := fakeHome(t)
	for _, c := range []struct {
		name, args string
		want       error
	}{
		{"montagem sem destino", "-sandbox-mount /a", errUsage},
		{"modo HTTP", "-upstream http://x/mcp", errSandboxMode},
		{
			"home montada", "-sandbox-mount " + home + ":/data -- /srv",
			sandbox.ErrMount,
		},
	} {
		args := append([]string{"-sandbox", "img"},
			strings.Fields(c.args)...)
		_, err := sandboxProfile(args...)
		t.Logf("%-20s recusada=%v", c.name, errors.Is(err, c.want))
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v; quer %v", c.name, err, c.want)
		}
	}
}

// #endregion

func TestSandbox_perfil(t *testing.T) {
	data := t.TempDir()
	p, err := sandboxProfile("-sandbox", "img", "-sandbox-mount",
		data+":/data", "--", "/srv")
	if err != nil || len(p.Mounts) != 1 || p.Mounts[0].Writable {
		t.Errorf("perfil = %+v, %v", p, err)
	}
	if p, _ := sandboxProfile("--", "/srv"); p != nil {
		t.Error("sem -sandbox, não há perfil")
	}
}

// Sem HOME, a Guarita não sabe onde ficam as pastas de credenciais, e
// uma montagem passaria sem a conferência delas: a partida é recusada.
// Sem montagens, não há o que conferir, e a sandbox sobe.
func TestSandbox_semHome(t *testing.T) {
	data := t.TempDir()
	t.Setenv("HOME", "")
	_, err := sandboxProfile("-sandbox", "img",
		"-sandbox-mount", data+":/data", "--", "/srv")
	t.Logf("com montagem: %v", err)
	if !errors.Is(err, errSandboxHome) {
		t.Errorf("quer %v, veio %v", errSandboxHome, err)
	}
	_, err = sandboxProfile("-sandbox", "img", "--", "/srv")
	if err != nil {
		t.Errorf("sem montagem: %v", err)
	}
}
