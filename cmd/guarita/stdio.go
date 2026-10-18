package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sandbox"
)

// stdioServer é o servidor do modo stdio: o comando e, se houver, a
// sandbox em que ele roda.
type stdioServer struct {
	command []string
	sandbox sandboxConfig
}

func (c config) stdio() stdioServer {
	return stdioServer{command: c.command, sandbox: c.sandbox}
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
// newServerCmd cria o processo do servidor: no host, como até o
// Capítulo 16, ou, com -sandbox, um docker run com o perfil da
// Guarita. O nome do contêiner volta para a remoção no fim.
func newServerCmd(
	ctx context.Context, target stdioServer,
) (*exec.Cmd, string, error) {
	profile, err := target.sandbox.profile()
	if err != nil {
		return nil, "", err
	}
	if profile != nil {
		return profile.Command(ctx, target.command)
	}
	command := target.command
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
