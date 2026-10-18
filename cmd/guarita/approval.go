package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/admin"
	"github.com/aluiziodeveloper/guarita/internal/adapters/approval"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

// approvalConfig é a aprovação humana do modo HTTP.
type approvalConfig struct {
	operators  string // arquivo "nome segredo"; vazio desliga
	listen     string // escuta do operador, só em loopback
	timeout    time.Duration
	maxPending int
	cooldown   time.Duration
}

// approvalFlags declara as opções da aprovação humana.
func approvalFlags(flags *flag.FlagSet, c *approvalConfig) {
	flags.StringVar(&c.operators, "operators", "",
		"arquivo dos operadores que aprovam (nome segredo por linha)")
	flags.StringVar(&c.listen, "admin-listen", "127.0.0.1:8081",
		"escuta da interface do operador")
	flags.DurationVar(&c.timeout, "approval-timeout", 2*time.Minute,
		"prazo de cada pedido de aprovação")
	flags.IntVar(&c.maxPending, "approval-max-pending", 3,
		"pedidos pendentes por agente")
	flags.DurationVar(&c.cooldown, "approval-cooldown", 10*time.Minute,
		"quanto tempo a mesma chamada negada é negada na hora")
}

// deciders são os que decidem: a política e, quando ela pede
// aprovação, uma pessoa, pela fila; no caminho de volta, o
// classificador.
type deciders struct {
	policy     app.PolicyEvaluator
	queue      *approval.Queue     // nil sem -operators
	classifier app.ResultInspector // nil sem -classifier
	quota      app.Inspector       // nil sem -quota nem -budget
}

// options devolve a opção do aprovador, se houver fila.
func (d deciders) options() []app.Option {
	if d.queue == nil {
		return nil
	}
	return []app.Option{app.WithApprover(d.queue)}
}

// newQueue cria a fila se houver operadores para decidir.
func newQueue(c approvalConfig) *approval.Queue {
	if c.operators == "" {
		return nil
	}
	return approval.New(c.timeout, approval.WithMaxPending(c.maxPending),
		approval.WithCooldown(c.cooldown))
}

// #region cmd-guarita-start-admin
// startAdmin abre a interface do operador noutra escuta, só em
// loopback, com três camadas: a guarda de Host do Capítulo 7, a
// proteção contra pedidos de outra origem e a credencial do operador.
func startAdmin(
	c approvalConfig, queue *approval.Queue, stderr io.Writer,
) (func(), error) {
	if queue == nil {
		return func() {}, nil
	}
	operators, err := loadOperators(c.operators)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", c.listen)
	if err != nil {
		return nil, err
	}
	handler, err := adminHandler(listener.Addr().String(), queue,
		operators)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	server := newServer(handler)
	go func() { _ = server.Serve(listener) }()
	_, _ = fmt.Fprintf(stderr, "guarita: aprovações em http://%s/\n",
		listener.Addr())
	return func() { _ = server.Close() }, nil
}

// #endregion

// adminHandler põe as proteções na frente das rotas do operador. Fora
// do loopback, LoopbackHosts falha, e a Guarita não sobe.
func adminHandler(
	addr string, queue *approval.Queue, operators admin.Operators,
) (http.Handler, error) {
	hosts, err := mcphttp.LoopbackHosts(addr)
	if err != nil {
		return nil, fmt.Errorf("-admin-listen must be loopback: %w", err)
	}
	guard, err := mcphttp.NewGuard(hosts, nil)
	if err != nil {
		return nil, err
	}
	csrf := http.NewCrossOriginProtection()
	return guard.Wrap(csrf.Handler(admin.NewHandler(queue, operators))),
		nil
}

// loadOperators lê o arquivo de operadores com as permissões do
// Capítulo 8: legível por outros, ele é recusado.
func loadOperators(path string) (admin.Operators, error) {
	content, err := secret.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return admin.ParseOperators(content.Reveal())
}
