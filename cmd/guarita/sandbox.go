package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
)

var (
	errSandboxMount = errors.New(
		"-sandbox-mount must be source:target or source:target:rw")
	errSandboxMode = errors.New(
		"-sandbox is for a stdio server, after --; HTTP servers are remote")
	errSandboxHome = errors.New(
		"-sandbox-mount needs a home directory (HOME) to check mounts")
)

// defenseFlags declara as opções das defesas dos Capítulos 12 a 21: a
// aprovação, os inspetores, a sandbox, os limites, a auditoria, o
// digest do servidor, os padrões seguros e a escuta de operação.
func defenseFlags(flags *flag.FlagSet, c *config) {
	approvalFlags(flags, &c.approval)
	inspectFlags(flags, &c.inspect)
	sandboxFlags(flags, &c.sandbox)
	limitsFlags(flags, &c.limits)
	auditFlags(flags, &c.audit)
	artifactFlags(flags, c)
	secureFlags(flags, c)
	opsFlags(flags, &c.ops)
}

// sandboxConfig põe o servidor stdio num contêiner.
type sandboxConfig struct {
	image  string // vazio: o servidor roda no host, como até aqui
	mounts []sandbox.Mount
}

// #region cmd-guarita-sandbox-flags
// sandboxFlags declara as opções da sandbox. Cada montagem é só de
// leitura, a menos que termine em :rw.
func sandboxFlags(flags *flag.FlagSet, c *sandboxConfig) {
	flags.StringVar(&c.image, "sandbox", "",
		"imagem em que o servidor stdio roda isolado")
	flags.Func("sandbox-mount", "diretório para o servidor: "+
		"origem:destino[:rw]", func(value string) error {
		parts := strings.Split(value, ":")
		if len(parts) < 2 || len(parts) > 3 ||
			(len(parts) == 3 && parts[2] != "rw") {
			return errSandboxMount
		}
		c.mounts = append(c.mounts, sandbox.Mount{
			Source: parts[0], Target: parts[1],
			Writable: len(parts) == 3,
		})
		return nil
	})
}

// #endregion

// #region cmd-guarita-sandbox-profile
// profile monta o perfil da sandbox, com cada montagem conferida pelo
// caminho real. Sem -sandbox, não há perfil, e o servidor roda no host.
func (c sandboxConfig) profile() (*sandbox.Profile, error) {
	if c.image == "" {
		return nil, nil
	}
	// Sem a home, CheckMount não saberia onde ficam .ssh, .aws e as
	// outras pastas de credenciais, nem recusaria a home inteira: com
	// montagens, a partida é recusada.
	home, err := os.UserHomeDir()
	if err != nil && len(c.mounts) > 0 {
		return nil, fmt.Errorf("%w: %w", errSandboxHome, err)
	}
	p := sandbox.Default(c.image)
	for _, m := range c.mounts {
		checked, err := sandbox.CheckMount(m, home)
		if err != nil {
			return nil, err
		}
		p.Mounts = append(p.Mounts, checked)
	}
	return &p, p.Validate()
}

// #endregion
