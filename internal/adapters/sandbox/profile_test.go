package sandbox

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// #region sandbox-args-test
// O perfil padrão, com uma montagem, vira um docker run com init, sem
// rede, só leitura, sem root, sem capabilities, com limites e com a
// montagem só de leitura.
func TestArgs(t *testing.T) {
	p := Default("guarita-lab:cap17")
	p.Mounts = []Mount{{Source: "/srv/dados", Target: "/data"}}
	command := []string{"/lab-files", "-root", "/data"}
	args := p.Args("guarita-sandbox-x", command)
	t.Log("docker " + strings.Join(args, " "))
	mount := "type=bind,source=/srv/dados,target=/data,readonly"
	for _, want := range [][]string{
		{"--init"},
		{"--network", "none"},
		{"--read-only"},
		{"--user", "65532:65532"},
		{"--cap-drop", "ALL"},
		{"--security-opt", "no-new-privileges=true"},
		{"--pids-limit", "64"},
		{"--memory", "256m"},
		{"--mount", mount},
	} {
		i := slices.Index(args, want[0])
		if i < 0 || !slices.Equal(args[i:i+len(want)], want) {
			t.Errorf("falta %v", want)
		}
	}
}

// #endregion

// #region sandbox-validate-test
// Root, imagem que parece opção, sem limites e uma montagem com
// vírgula, que acrescentaria uma opção à de --mount, são recusados.
func TestValidate(t *testing.T) {
	comma := []Mount{{Source: "/a,readonly=false", Target: "/b"}}
	for _, c := range []struct {
		name   string
		change func(*Profile)
	}{
		{"root", func(p *Profile) { p.User = "0:0" }},
		{"sem usuário", func(p *Profile) { p.User = "" }},
		{"imagem-opção", func(p *Profile) { p.Image = "--privileged" }},
		{"sem limites", func(p *Profile) { p.PIDs = 0 }},
		{"vírgula", func(p *Profile) { p.Mounts = comma }},
	} {
		p := Default("img")
		c.change(&p)
		err := p.Validate()
		t.Logf("%-13s %v", c.name, err)
		if !errors.Is(err, ErrProfile) {
			t.Errorf("%s: aceito", c.name)
		}
	}
	if err := Default("img").Validate(); err != nil {
		t.Errorf("perfil padrão recusado: %v", err)
	}
}

// #endregion

func TestArgs_imagemAntesDoComando(t *testing.T) {
	args := Default("img").Args("n", []string{"/srv", "-x"})
	if !slices.Equal(args[len(args)-3:], []string{"img", "/srv", "-x"}) {
		t.Errorf("fim dos argumentos: %v", args[len(args)-3:])
	}
}
