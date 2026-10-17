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
	"strings"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/identity"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
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
// serveHTTP escuta, monta o endpoint e atende até ctx acabar. Aí para
// de aceitar conexões e espera as chamadas em curso, com prazo.
func serveHTTP(
	ctx context.Context, c config, policy app.PolicyEvaluator,
	stderr io.Writer,
) error {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	catalog, closeCatalog, err := openCatalog(ctx, c, logger)
	if err != nil {
		return err
	}
	defer closeCatalog()
	listener, err := net.Listen("tcp", c.listen)
	if err != nil {
		return err
	}
	endpoint, err := newEndpoint(c, policy, catalog,
		listener.Addr().String(), logger)
	if err != nil {
		_ = listener.Close()
		return err
	}
	server := newServer(endpoint)
	_, _ = fmt.Fprintf(stderr, "guarita: escutando em http://%s/mcp\n",
		listener.Addr())
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	return shutdown(ctx, server, served)
}

// #endregion

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
) error {
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
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
// newEndpoint compõe o endpoint: a guarda de Host e Origin na frente de
// tudo, a identidade do agente em /mcp e os metadados do recurso.
func newEndpoint(
	c config, policy app.PolicyEvaluator, catalog *app.PinCatalog,
	addr string, logger *slog.Logger,
) (http.Handler, error) {
	resource := cmp.Or(c.auth.resource, "http://"+addr+"/mcp")
	require, metadata, err := newAuth(c, resource, logger)
	if err != nil {
		return nil, err
	}
	handler, err := newHandler(c, policy, catalog, logger)
	if err != nil {
		return nil, err
	}
	guard, err := newGuard(c, addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", require(handler))
	if metadata != nil {
		mux.Handle(identity.MetadataPath, metadata)
	}
	return guard.Wrap(mux), nil
}

// #endregion

// #region cmd-guarita-new-auth
// newAuth monta a verificação do agente. Sem -agent-key, ela só é
// aceita em loopback, e o registro avisa; fora dele, a partida falha.
func newAuth(c config, resource string, logger *slog.Logger) (
	identity.Middleware, http.Handler, error,
) {
	if c.auth.keyFile == "" {
		if len(c.hosts) > 0 {
			return nil, nil, errAuthRequired
		}
		logger.Warn("sem autenticação do agente; aceita só em loopback")
		return passThrough, nil, nil
	}
	verifier, err := loadVerifier(c.auth, resource)
	if err != nil {
		return nil, nil, err
	}
	metadataURL := strings.TrimSuffix(resource, "/mcp") +
		identity.MetadataPath
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

// passThrough é o middleware que não faz nada, para o loopback sem
// autenticação.
func passThrough(next http.Handler) http.Handler { return next }

// newHandler monta o proxy com a credencial da Guarita, se houver.
func newHandler(
	c config, policy app.PolicyEvaluator, catalog *app.PinCatalog,
	logger *slog.Logger,
) (*mcphttp.Handler, error) {
	upstream, err := url.Parse(c.upstream)
	if err != nil {
		return nil, err
	}
	options := []mcphttp.Option{
		mcphttp.WithCallTimeout(c.callTimeout),
		mcphttp.WithLogger(logger),
		mcphttp.WithAgent(identity.Subject),
	}
	if c.auth.credentialFile != "" {
		credential, err := secret.ReadFile(c.auth.credentialFile)
		if err != nil {
			return nil, err
		}
		options = append(options,
			mcphttp.WithUpstreamCredential(credential))
	}
	var useCase []app.Option
	if catalog != nil { // o mesmo catálogo filtra a lista e nega a chamada
		useCase = append(useCase, app.WithCatalog(catalog))
		options = append(options, mcphttp.WithCatalog(catalog))
	}
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil, useCase...))
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
// openCatalog abre o catálogo do servidor em -upstream. Sem -catalog, a
// Guarita segue sem ele e avisa: qualquer definição chega ao agente.
func openCatalog(ctx context.Context, c config, logger *slog.Logger) (
	*app.PinCatalog, func(), error,
) {
	if c.catalogFile == "" {
		logger.Warn("sem catálogo; as definições não são fixadas")
		return nil, func() {}, nil
	}
	store, err := sqlitestore.Open(ctx, c.catalogFile)
	if err != nil {
		return nil, nil, err
	}
	closeStore := func() { _ = store.Close() }
	return app.NewPinCatalog(store, c.upstream), closeStore, nil
}

// #endregion
