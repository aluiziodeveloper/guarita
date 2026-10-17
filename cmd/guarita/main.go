package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

const usage = "uso: guarita [-allow ferramenta,…] -- " +
	"<comando do servidor MCP> [argumentos]\n" +
	"     guarita [-allow …] [-listen endereço] [-host …] [-origin …] " +
	"-upstream <URL do servidor MCP>"

var errUsage = errors.New(usage)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	if err != nil {
		log.Printf("guarita: %v", err)
		os.Exit(1)
	}
}

// #region cmd-guarita-run
// run é a raiz de composição: lê a configuração, monta a política, o
// caso de uso e a porta, e escolhe o transporte.
func run(
	ctx context.Context,
	args []string,
	stdin io.ReadCloser,
	stdout io.WriteCloser,
	stderr io.Writer,
) error {
	config, err := parseConfig(args, stderr)
	if err != nil {
		return err
	}
	policy, err := policystatic.New(config.decisions)
	if err != nil {
		return err
	}
	// Os dois transportes só usam Authorize: o repasse fica com eles.
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil))
	if config.upstream != "" {
		return serveHTTP(ctx, config, gate, stderr)
	}
	return serveStdio(ctx, config.command, gate, stdin, stdout, stderr)
}

// #endregion

type config struct {
	decisions   map[string]domain.Decision
	command     []string // modo stdio
	upstream    string   // modo HTTP
	listen      string
	hosts       []string
	origins     []string
	callTimeout time.Duration
	auth        authConfig
}

// authConfig é a identidade do modo HTTP: quem emite os tokens do
// agente, para que recurso, e a credencial da Guarita para o servidor.
type authConfig struct {
	keyFile        string
	issuer         string
	resource       string
	credentialFile string
}

// #region cmd-guarita-parse-config
// parseConfig lê as opções antes de "--" e o comando do servidor depois
// dele. -upstream escolhe o modo HTTP; os dois modos juntos são erro.
func parseConfig(args []string, stderr io.Writer) (config, error) {
	flagArgs, command := args, []string(nil)
	if i := slices.Index(args, "--"); i >= 0 {
		flagArgs, command = args[:i], args[i+1:]
	}
	flags := flag.NewFlagSet("guarita", flag.ContinueOnError)
	flags.SetOutput(stderr)
	allow := flags.String("allow", "", "ferramentas permitidas")
	c := config{command: command}
	hosts, origins := httpFlags(flags, &c)
	if err := flags.Parse(flagArgs); err != nil {
		return config{}, errors.Join(errUsage, err)
	}
	if flags.NArg() > 0 || (c.upstream == "") == (len(command) == 0) {
		return config{}, errUsage
	}
	c.decisions = allowAll(list(*allow))
	c.hosts, c.origins = list(*hosts), list(*origins)
	return c, nil
}

// #endregion

// #region cmd-guarita-http-flags
// httpFlags declara as opções do modo HTTP. As listas voltam como texto
// e são separadas depois de Parse.
func httpFlags(
	flags *flag.FlagSet, c *config,
) (hosts, origins *string) {
	flags.StringVar(&c.upstream, "upstream", "", "URL do servidor HTTP")
	flags.StringVar(&c.listen, "listen", "127.0.0.1:8080", "escuta")
	hosts = flags.String("host", "", "Host aceitos (padrão: loopback)")
	origins = flags.String("origin", "", "origens aceitas")
	flags.DurationVar(&c.callTimeout, "call-timeout",
		mcphttp.DefaultCallTimeout, "prazo de cada chamada")
	flags.StringVar(&c.auth.keyFile, "agent-key", "",
		"chave pública Ed25519 (PEM) do emissor dos tokens do agente")
	flags.StringVar(&c.auth.issuer, "agent-issuer", "",
		"emissor dos tokens do agente")
	flags.StringVar(&c.auth.resource, "resource", "",
		"recurso protegido (padrão: http://<escuta>/mcp)")
	flags.StringVar(&c.auth.credentialFile, "upstream-credential", "",
		"arquivo com a credencial da Guarita para o servidor")
	return hosts, origins
}

// #endregion

// allowAll permite as ferramentas listadas; as outras caem na falha
// fechada de policystatic.
func allowAll(tools []string) map[string]domain.Decision {
	decisions := make(map[string]domain.Decision, len(tools))
	for _, tool := range tools {
		decisions[tool] = domain.DecisionAllow
	}
	return decisions
}

// list separa uma opção por vírgulas, sem itens vazios.
func list(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
