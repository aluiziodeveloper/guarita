package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/identity"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/scanner"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sqlitestore"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

// errAuthRequired recusa a escuta fora do loopback sem identidade.
var errAuthRequired = errors.New(
	"-agent-key is required when -host is set")

// httpShutdownGrace é quanto o encerramento espera as chamadas em curso.
const httpShutdownGrace = 5 * time.Second

// #region cmd-guarita-serve-http
// serveHTTP confere os padrões, abre as escutas e atende até ctx
// acabar; aí o /readyz vira 503, e as chamadas em curso terminam.
func serveHTTP(
	ctx context.Context, c config, d deciders,
	stderr io.Writer,
) error {
	if err := c.checkDefaults(); err != nil {
		return err
	}
	logger := newLogger(stderr)
	var ready atomic.Bool
	store, closeSides, err := openSides(ctx, c, d, &ready, logger,
		stderr)
	if err != nil {
		return err
	}
	defer closeSides()
	listener, endpoint, err := listen(ctx, c, d, store, logger)
	if err != nil {
		return err
	}
	server := newServer(endpoint)
	_, _ = fmt.Fprintf(stderr, "guarita: escutando em %s://%s%s\n",
		c.scheme(), listener.Addr(), c.servers[0].path())
	served := make(chan error, 1)
	go func() { served <- c.serve(server, listener) }()
	ready.Store(true)
	return shutdown(ctx, server, served, &ready)
}

// #endregion

// #region cmd-guarita-open-sides
// openSides abre o que fica ao lado da escuta do agente: a operação, o
// catálogo e a página do operador. closeAll fecha na ordem inversa.
func openSides(
	ctx context.Context, c config, d deciders, ready *atomic.Bool,
	logger *slog.Logger, stderr io.Writer,
) (app.CatalogStore, func(), error) {
	stopOps, err := startOps(c.ops, d.metrics, ready, stderr)
	if err != nil {
		return nil, nil, err
	}
	store, closeStore, err := openStore(ctx, c, logger)
	if err != nil {
		stopOps()
		return nil, nil, err
	}
	stopAdmin, err := startAdmin(c.approval, d.queue, stderr)
	if err != nil {
		closeStore()
		stopOps()
		return nil, nil, err
	}
	return store, func() { stopAdmin(); closeStore(); stopOps() }, nil
}

// #endregion

// listen prepara o classificador, se houver, abre a escuta do agente
// e monta o endpoint para o endereço que ela recebeu.
func listen(
	ctx context.Context, c config, d deciders, store app.CatalogStore,
	logger *slog.Logger,
) (net.Listener, http.Handler, error) {
	var err error
	d.classifier, err = newClassifier(ctx, c.inspect.classifier, logger)
	if err != nil {
		return nil, nil, err
	}
	if d.quota, err = c.limits.newQuota(); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", c.listen)
	if err != nil {
		return nil, nil, err
	}
	endpoint, err := newEndpoint(c, d, store, listener.Addr().String(),
		logger)
	if err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	return listener, endpoint, nil
}

// newServer dá ao servidor os prazos de conexão. Sem WriteTimeout: ele
// cortaria os fluxos longos; o prazo de cada chamada fica no handler.
func newServer(endpoint http.Handler) *http.Server {
	return &http.Server{
		Handler:           endpoint,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
}

// #region cmd-guarita-shutdown
// shutdown espera o fim de ctx ou do servidor. Shutdown não interrompe
// fluxos abertos; esgotado o prazo, Close os corta.
func shutdown(
	ctx context.Context, server *http.Server, served <-chan error,
	ready *atomic.Bool,
) error {
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	ready.Store(false) // o /readyz passa a 503 antes de parar
	grace, cancel := context.WithTimeout(context.WithoutCancel(ctx),
		httpShutdownGrace)
	defer cancel()
	err := server.Shutdown(grace)
	if err != nil {
		_ = server.Close()
	}
	<-served // a goroutine de Serve termina antes do retorno
	return errors.Join(ctx.Err(), err)
}

// #endregion

// #region cmd-guarita-new-endpoint
// newEndpoint compõe o endpoint: a guarda na frente de tudo; cada
// servidor no seu caminho, com identidade, catálogo e metadados.
func newEndpoint(
	c config, d deciders, store app.CatalogStore,
	addr string, logger *slog.Logger,
) (http.Handler, error) {
	guard, err := newGuard(c, addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	named := routes{}
	for _, server := range c.servers {
		route, metadata, err := newRoute(c, server, d, store, addr,
			logger)
		if err != nil {
			return nil, err
		}
		if server.name == "" {
			mux.Handle("/mcp", route)
			mux.Handle(identity.MetadataPath, metadata)
			continue
		}
		named[server.name] = route
		mux.Handle(identity.MetadataPath+server.path(), metadata)
	}
	mux.Handle("/mcp/{server}", named)
	return d.measured(guard.Wrap(mux)), nil
}

// #endregion

// #region cmd-guarita-new-route
// newRoute monta um servidor: o recurso protegido é o caminho dele, e
// um token emitido para outro servidor da Guarita não serve aqui. Com
// -host, o recurso usa o nome público, e não o endereço da escuta.
func newRoute(
	c config, server upstreamServer, d deciders,
	store app.CatalogStore, addr string, logger *slog.Logger,
) (http.Handler, http.Handler, error) {
	base := c.scheme() + "://" + cmp.Or(first(c.hosts), addr)
	resource := base + server.path()
	metadataURL := base + identity.MetadataPath
	if server.name == "" {
		resource = cmp.Or(c.auth.resource, resource)
	} else {
		metadataURL += server.path()
	}
	require, metadata, err := newAuth(c, resource, metadataURL, logger)
	if err != nil {
		return nil, nil, err
	}
	handler, err := newHandler(c, server, d, store, logger)
	if err != nil {
		return nil, nil, err
	}
	return require(handler), metadata, nil
}

// #endregion

// #region cmd-guarita-new-auth
// newAuth monta a verificação do agente. Sem -agent-key, ela só é
// aceita em loopback, e o registro avisa; fora dele, a partida falha.
func newAuth(
	c config, resource, metadataURL string, logger *slog.Logger,
) (identity.Middleware, http.Handler, error) {
	if c.auth.keyFile == "" {
		if len(c.hosts) > 0 {
			return nil, nil, errAuthRequired
		}
		if !c.auth.insecure {
			return nil, nil, errNoAuth
		}
		logger.Warn("sem autenticação do agente; aceita só em loopback",
			"resource", resource)
		return passThrough, http.NotFoundHandler(), nil
	}
	verifier, err := loadVerifier(c.auth, resource)
	if err != nil {
		return nil, nil, err
	}
	return identity.Require(verifier, metadataURL),
		identity.Metadata(resource, c.auth.issuer), nil
}

// #endregion

// loadVerifier lê a chave pública do emissor e monta o verificador.
func loadVerifier(auth authConfig, resource string) (
	*identity.Verifier, error,
) {
	//nolint:gosec // G304: o caminho vem da configuração do operador
	data, err := os.ReadFile(auth.keyFile)
	if err != nil {
		return nil, err
	}
	key, err := identity.ParsePublicKey(data)
	if err != nil {
		return nil, err
	}
	return identity.NewVerifier(key, auth.issuer, resource)
}

// first é o primeiro item, ou vazio.
func first(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

// passThrough é o middleware que não faz nada, para o loopback sem
// autenticação.
func passThrough(next http.Handler) http.Handler { return next }

// newHandler monta o proxy com a credencial da Guarita, se houver.
func newHandler(
	c config, server upstreamServer, d deciders,
	store app.CatalogStore, logger *slog.Logger,
) (*mcphttp.Handler, error) {
	upstream, err := url.Parse(server.url)
	if err != nil {
		return nil, err
	}
	options := []mcphttp.Option{
		mcphttp.WithCallTimeout(c.callTimeout),
		mcphttp.WithLogger(logger),
		mcphttp.WithAgent(identity.Subject),
	}
	if d.tracer != nil {
		options = append(options, mcphttp.WithTracer(d.tracer))
	}
	rt := c.limits.transport(server.name, logger)
	if d.metrics != nil {
		rt = d.metrics.Transport(rt)
	}
	if rt != nil {
		options = append(options, mcphttp.WithTransport(rt))
	}
	if server.credential != "" {
		credential, err := secret.ReadFile(server.credential)
		if err != nil {
			return nil, err
		}
		options = append(options,
			mcphttp.WithUpstreamCredential(credential))
	}
	useCase := []app.Option{ // a política sabe onde e quem
		app.WithServer(server.name), app.WithAgent(identity.Subject),
	}
	var catalog *app.PinCatalog
	if store != nil { // o mesmo catálogo filtra a lista e nega a chamada
		catalog = app.NewPinCatalog(store, server.catalogKey(),
			app.WithScanner(scanner.Structural{}),
			app.WithRejections(logRejection(logger)))
		useCase = append(useCase, app.WithCatalog(catalog))
		options = append(options, mcphttp.WithCatalog(catalog))
	}
	line, err := routeLine(c.inspect, server.name, catalog, d, logger)
	if err != nil {
		return nil, err
	}
	useCase = append(useCase, line.Options()...)
	useCase = append(useCase, app.WithFindings(logFindings(logger)))
	useCase = append(useCase, d.options()...) // a fila, se houver
	useCase = append(useCase, d.audited()...) // a trilha, se houver
	uc := app.NewInterceptCall(d.policy, nil, useCase...)
	if line.ReadsResults() { // o caminho de volta
		options = append(options, mcphttp.WithResults(uc))
	}
	var gate mcphttp.Admitter = mcpproxy.NewGate(uc)
	if d.metrics != nil {
		gate = d.metrics.Admitter(gate)
	}
	return mcphttp.NewHandler(upstream, gate, options...)
}

// newGuard aceita os hosts de -host ou, sem eles, os do loopback.
func newGuard(c config, addr string) (*mcphttp.Guard, error) {
	hosts := c.hosts
	if len(hosts) == 0 {
		var err error
		if hosts, err = mcphttp.LoopbackHosts(addr); err != nil {
			return nil, err // fora do loopback, -host é obrigatório
		}
	}
	return mcphttp.NewGuard(hosts, c.origins)
}

// #region cmd-guarita-open-catalog
// openStore abre o banco do catálogo, um só para todos os servidores:
// é nele que a colisão de nomes entre eles aparece. Sem -catalog, a
// Guarita avisa que não fixa definições.
func openStore(ctx context.Context, c config, logger *slog.Logger) (
	app.CatalogStore, func(), error,
) {
	if c.catalogFile == "" {
		logger.Warn("sem catálogo; as definições não são fixadas")
		return nil, func() {}, nil
	}
	store, err := sqlitestore.Open(ctx, c.catalogFile)
	if err != nil {
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

// #endregion
