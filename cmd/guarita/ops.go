package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"sync/atomic"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/metrics"
)

var (
	errPprofLoopback = errors.New("-pprof requires a loopback -ops-listen")
	errUnhealthy     = errors.New("guarita não está saudável")
)

// opsConfig é a escuta de operação: métricas, saúde e, se pedido, os
// perfis. Ela é separada da escuta do agente, e vazia por padrão.
type opsConfig struct {
	listen string
	pprof  bool
}

// #region cmd-guarita-ops-flags
// opsFlags declara a escuta de operação. O pprof só é aceito nela, e
// só no loopback: os perfis mostram a memória e a linha de comando.
func opsFlags(flags *flag.FlagSet, c *opsConfig) {
	flags.StringVar(&c.listen, "ops-listen", "",
		"escuta de /metrics, /healthz e /readyz (padrão: nenhuma)")
	flags.BoolVar(&c.pprof, "pprof", false,
		"expõe /debug/pprof na escuta de operação, só em loopback")
}

// #endregion

// #region cmd-guarita-ops-mux
// opsMux monta as rotas de operação num multiplexador próprio: o
// DefaultServeMux, em que o import de net/http/pprof se registra, nunca
// é servido. No loopback, a guarda de Host do Capítulo 7 vale aqui
// também, com o endereço que a escuta recebeu.
func (c opsConfig) opsMux(
	addr string, m *metrics.Metrics, ready *atomic.Bool,
) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m)
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(ready))
	hosts, err := mcphttp.LoopbackHosts(addr)
	if err != nil {
		if c.pprof {
			return nil, errPprofLoopback
		}
		return mux, nil // fora do loopback, como o probe de um cluster
	}
	if c.pprof {
		profiles(mux)
	}
	guard, err := mcphttp.NewGuard(hosts, nil)
	if err != nil {
		return nil, err
	}
	return guard.Wrap(mux), nil
}

// #endregion

// healthz responde enquanto o processo atende pedidos.
func healthz(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, "ok\n")
}

// profiles registra as rotas do pprof uma a uma, no mux dado.
func profiles(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// readyz responde 200 enquanto a Guarita atende e 503 antes e durante o
// encerramento, para o balanceador parar de mandar pedidos.
func readyz(ready *atomic.Bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "not ready\n")
			return
		}
		_, _ = io.WriteString(w, "ready\n")
	}
}

// #region cmd-guarita-start-ops
// startOps abre a escuta de operação, se houver uma.
func startOps(
	c opsConfig, m *metrics.Metrics, ready *atomic.Bool,
	stderr io.Writer,
) (func(), error) {
	if c.listen == "" {
		return func() {}, nil
	}
	listener, err := net.Listen("tcp", c.listen)
	if err != nil {
		return nil, err
	}
	handler, err := c.opsMux(listener.Addr().String(), m, ready)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	server := newServer(handler)
	go func() { _ = server.Serve(listener) }()
	_, _ = fmt.Fprintf(stderr, "guarita: operação em http://%s/\n",
		listener.Addr())
	return func() { _ = server.Close() }, nil
}

// #endregion

// #region cmd-guarita-health
// runHealth é o probe para uma imagem sem shell: pede a URL e falha se
// a resposta não for 200. É o que o healthcheck do Compose executa.
func runHealth(
	ctx context.Context, args []string, out, stderr io.Writer,
) error {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	flags.SetOutput(stderr)
	url := flags.String("url", "http://127.0.0.1:9090/healthz", "URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", *url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", errUnhealthy, res.Status)
	}
	_, err = fmt.Fprintln(out, res.Status)
	return err
}

// #endregion

// metrics cria as medidas, se houver escuta de operação para expô-las.
func (c opsConfig) metrics() *metrics.Metrics {
	if c.listen == "" {
		return nil
	}
	return metrics.New()
}

// measured envolve o endpoint nas medidas, se houver: por fora de
// tudo, para medir a Guarita inteira, guarda e identidade incluídas.
func (d deciders) measured(endpoint http.Handler) http.Handler {
	if d.metrics == nil {
		return endpoint
	}
	return d.metrics.Middleware(endpoint)
}
