package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var (
	errStdioFlag    = errors.New("option needs the HTTP mode (-server)")
	errStdioResults = errors.New(
		"stdio does not review results; the line reads them")
)

// #region cmd-guarita-stdio-flags
// stdioFlags são as opções que o modo stdio aplica: a política, a
// trilha, a sandbox, o digest e a linha de ida. É uma lista de
// permissão: uma opção nova fica de fora até alguém a ligar aqui.
var stdioFlags = []string{
	"allow", "policy", "audit", "audit-key",
	"sandbox", "sandbox-mount", "server-digest",
	"dlp", "dlp-entropy", "allow-hosts",
	"quota", "cost", "budget", "pipeline", "require-stages",
}

// checkStdioFlags recusa, no modo stdio, as opções escritas que ele não
// aplicaria. Visit só passa pelas opções que a linha de comando deu.
func checkStdioFlags(flags *flag.FlagSet) error {
	var refused []string
	flags.Visit(func(f *flag.Flag) {
		if !slices.Contains(stdioFlags, f.Name) {
			refused = append(refused, "-"+f.Name)
		}
	})
	if len(refused) > 0 {
		return fmt.Errorf("%w: %s", errStdioFlag,
			strings.Join(refused, " "))
	}
	return nil
}

// #endregion

// #region cmd-guarita-stdio-use-case
// stdioUseCase monta o caso de uso do modo stdio com a linha de ida das
// rotas HTTP: DLP, destinos e cota. A volta não: o repasse do stdio não
// abre as respostas, e uma etapa de volta impede a partida.
func stdioUseCase(
	c config, d deciders, stderr io.Writer,
) (*app.InterceptCall, error) {
	if err := c.checkRoutes(); err != nil {
		return nil, err
	}
	var err error
	if d.quota, err = c.limits.newQuota(); err != nil {
		return nil, err
	}
	logger := newLogger(stderr)
	line, err := routeLine(c.inspect, "", nil, d, logger)
	if err != nil {
		return nil, err
	}
	if line.ReadsResults() {
		return nil, fmt.Errorf("%w: %s", errStdioResults,
			strings.Join(line.Names, ","))
	}
	options := append(line.Options(),
		app.WithFindings(logFindings(logger)))
	options = append(options, d.audited()...)
	return app.NewInterceptCall(d.policy, nil, options...), nil
}

// #endregion

// stdioServer é o servidor do modo stdio: o comando e, se houver, a
// sandbox em que ele roda.
type stdioServer struct {
	command []string
	sandbox sandboxConfig
	digest  domain.Digest
}

func (c config) stdio() stdioServer {
	return stdioServer{c.command, c.sandbox, c.digest}
}

// #region cmd-guarita-serve-stdio
// serveStdio inicia o servidor como subprocesso e repassa as mensagens
// até ele fechar a saída ou ctx acabar.
func serveStdio(
	ctx context.Context,
	target stdioServer,
	gate mcpproxy.Admitter,
	stdin io.ReadCloser,
	stdout io.WriteCloser,
	stderr io.Writer,
) error {
	server, container, err := newServerCmd(ctx, target)
	if err != nil {
		return err
	}
	serverIO, err := startServer(server, stderr)
	if err != nil {
		return err
	}
	agent := mcpproxy.Endpoint{Reader: stdin, Writer: stdout}
	stats, relayErr := mcpproxy.Relay(ctx, agent, serverIO, gate)
	waitErr := server.Wait() // só depois que o repasse leu tudo
	report(stderr, stats)
	return errors.Join(relayErr, waitErr, removeContainer(container))
}

// #endregion

// #region cmd-guarita-new-server-cmd
// newServerCmd cria o processo do servidor, depois de conferi-lo: no
// host, como até o Capítulo 16, ou, com -sandbox, um docker run com o
// perfil da Guarita. O nome do contêiner volta para a remoção no fim.
func newServerCmd(
	ctx context.Context, target stdioServer,
) (*exec.Cmd, string, error) {
	command, err := target.verified()
	if err != nil {
		return nil, "", err
	}
	profile, err := target.sandbox.profile()
	if err != nil {
		return nil, "", err
	}
	if profile != nil {
		return profile.Command(ctx, command)
	}
	// G204 avisa sobre comando vindo de fora. Aqui ele vem do operador,
	// na linha de comando, nunca de uma mensagem do agente.
	//nolint:gosec // o operador escolhe o servidor
	return exec.CommandContext(ctx, command[0], command[1:]...), "", nil
}

// #endregion

// #region cmd-guarita-start-server
// startServer inicia o processo e devolve as pontas da entrada e da
// saída padrão dele.
func startServer(
	server *exec.Cmd, stderr io.Writer,
) (mcpproxy.Endpoint, error) {
	server.Stderr = stderr
	var none mcpproxy.Endpoint
	in, err := server.StdinPipe()
	if err != nil {
		return none, fmt.Errorf("server stdin: %w", err)
	}
	out, err := server.StdoutPipe()
	if err != nil {
		return none, fmt.Errorf("server stdout: %w", err)
	}
	if err := server.Start(); err != nil {
		return none, fmt.Errorf("start server: %w", err)
	}
	return mcpproxy.Endpoint{Reader: out, Writer: in}, nil
}

// #endregion

// removeContainer apaga o contêiner da sandbox, se houver um.
func removeContainer(name string) error {
	if name == "" {
		return nil
	}
	return sandbox.Remove(name)
}

func report(stderr io.Writer, stats mcpproxy.Stats) {
	_, _ = fmt.Fprintf(stderr, "guarita: %d do agente, %d do servidor, "+
		"%d recusadas\n", stats.AgentToServer, stats.ServerToAgent,
		stats.Rejected)
}
