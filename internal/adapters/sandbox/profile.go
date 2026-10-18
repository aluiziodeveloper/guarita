// Package sandbox isola um servidor MCP local num contêiner. Ele monta
// o comando docker run com o perfil da Guarita: sem rede, sistema de
// arquivos só de leitura, usuário sem privilégios, nenhuma capability,
// sem ganho de privilégio e com limites de memória e de processos. O
// perfil seccomp padrão do Docker continua valendo, porque nenhuma
// opção o troca.
package sandbox

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ErrProfile indica um perfil incompleto ou inseguro.
var ErrProfile = errors.New("sandbox: invalid profile")

// Mount é um diretório do host visto pelo contêiner. Só leitura, a
// menos que Writable diga o contrário.
type Mount struct {
	Source, Target string
	Writable       bool
}

// #region sandbox-profile
// Profile é o perfil do contêiner de um servidor. Os campos são o que
// o operador escolhe; o resto do isolamento é fixo, em Args.
type Profile struct {
	Image  string
	Mounts []Mount
	User   string // uid:gid, nunca root
	Memory string // no formato do Docker, como 256m
	PIDs   int    // processos e threads, no máximo
}

// Default é o perfil para a imagem dada: o usuário nonroot das
// imagens distroless, 256 MiB e 64 processos.
func Default(image string) Profile {
	return Profile{
		Image: image, User: "65532:65532", Memory: "256m", PIDs: 64,
	}
}

// #endregion

// #region sandbox-args
// Args devolve os argumentos de docker run para o comando do servidor,
// na ordem: o isolamento fixo, os limites, as montagens, a imagem e o
// comando. O ambiente do host não entra: o Docker só passa ao
// contêiner as variáveis dadas com -e, e o perfil não dá nenhuma.
// --init põe um processo mínimo como PID 1, que repassa os sinais.
func (p Profile) Args(name string, command []string) []string {
	args := []string{
		"run", "--rm", "-i", "--init", "--name", name,
		"--network", "none",
		"--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
		"--user", p.User,
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges=true",
		"--pids-limit", strconv.Itoa(p.PIDs),
		"--memory", p.Memory,
	}
	for _, m := range p.Mounts {
		spec := "type=bind,source=" + m.Source + ",target=" + m.Target
		if !m.Writable {
			spec += ",readonly"
		}
		args = append(args, "--mount", spec)
	}
	return append(append(args, p.Image), command...)
}

// #endregion

// #region sandbox-validate
// Validate recusa o perfil que desfaria o isolamento: sem imagem, com
// usuário root, sem limites ou com uma montagem cujo texto mudaria o
// sentido da opção --mount, como uma vírgula no caminho.
func (p Profile) Validate() error {
	uid, _, _ := strings.Cut(p.User, ":")
	switch {
	case p.Image == "" || strings.HasPrefix(p.Image, "-"):
		return fmt.Errorf("%w: image %q", ErrProfile, p.Image)
	case uid == "" || uid == "0" || uid == "root":
		return fmt.Errorf("%w: user %q", ErrProfile, p.User)
	case p.Memory == "" || p.PIDs <= 0:
		return fmt.Errorf("%w: limits", ErrProfile)
	}
	for _, m := range p.Mounts {
		if strings.ContainsAny(m.Source+m.Target, ",=\n") {
			return fmt.Errorf("%w: mount %q", ErrProfile, m.Source)
		}
	}
	return nil
}

// #endregion

// #region sandbox-command
// Command cria o docker run do servidor, com um nome sorteado. Quando
// ctx acaba, o cliente do Docker recebe SIGTERM, e não o SIGKILL que
// exec.CommandContext manda por padrão: ele repassa o SIGTERM ao
// contêiner, e o PID 1 de --init encerra o servidor. Um SIGKILL
// mataria só o cliente e deixaria o contêiner rodando.
func (p Profile) Command(
	ctx context.Context, command []string,
) (*exec.Cmd, string, error) {
	if err := p.Validate(); err != nil {
		return nil, "", err
	}
	name := "guarita-sandbox-" + strings.ToLower(rand.Text()[:12])
	//nolint:gosec // G204: o comando vem do operador, nunca do agente
	cmd := exec.CommandContext(ctx, "docker", p.Args(name, command)...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	return cmd, name, nil
}

// #endregion

// #region sandbox-remove
// Remove apaga o contêiner pelo nome, se ele ainda existir. É a última
// barreira contra o contêiner órfão: se o cliente do Docker morreu sem
// repassar o sinal, o contêiner continua rodando até alguém removê-lo.
func Remove(name string) error {
	//nolint:gosec // G204: o nome é o que Command sorteou
	remove := exec.Command("docker", "rm", "-f", name)
	out, err := remove.CombinedOutput()
	gone := strings.Contains(string(out), "No such container")
	if err != nil && !gone {
		return fmt.Errorf("sandbox: remove %s: %w: %s", name, err, out)
	}
	return nil
}

// #endregion
