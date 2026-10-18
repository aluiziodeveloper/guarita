package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics reúne as medidas de uma Guarita.
type Metrics struct {
	request, upstream, overhead *Histogram
	forwarded, denied           counter
	inFlight                    atomic.Int64
	stages                      sync.Map // nome da etapa -> *Histogram
}

// New cria as medidas, todas zeradas.
func New() *Metrics {
	return &Metrics{
		request: newHistogram("guarita_request_seconds",
			"Tempo de cada pedido do agente, do início ao fim."),
		upstream: newHistogram("guarita_upstream_seconds",
			"Tempo de cada ida e volta ao servidor."),
		overhead: newHistogram("guarita_overhead_seconds",
			"Tempo do pedido que não foi do servidor: a Guarita."),
		forwarded: counter{label: "forwarded"},
		denied:    counter{label: "denied"},
	}
}

// waitKey guarda, no contexto do pedido, quanto ele esperou o servidor.
type waitKey struct{}

// #region metrics-middleware
// Middleware mede cada pedido. O tempo do servidor vem do Transport,
// pelo contexto, que o repasse herda do pedido do agente; o que sobra é
// a latência que a Guarita acrescenta.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter, r *http.Request,
	) {
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)
		var waited atomic.Int64
		r = r.WithContext(context.WithValue(r.Context(), waitKey{},
			&waited))
		start := time.Now()
		next.ServeHTTP(w, r)
		total := time.Since(start)
		m.request.Observe(total)
		m.overhead.Observe(total - time.Duration(waited.Load()))
	})
}

// #endregion

// #region metrics-transport
// Transport mede cada ida e volta ao servidor e soma o tempo no pedido
// do agente que a originou: a espera pelos cabeçalhos e cada leitura do
// corpo, que num fluxo continua depois do RoundTrip. Sem next, usa o
// http.DefaultTransport.
func (m *Metrics) Transport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return roundTrip(func(r *http.Request) (*http.Response, error) {
		waited, _ := r.Context().Value(waitKey{}).(*atomic.Int64)
		start := time.Now()
		res, err := next.RoundTrip(r)
		d := time.Since(start)
		m.upstream.Observe(d)
		if waited == nil {
			return res, err
		}
		waited.Add(int64(d))
		if res != nil {
			res.Body = &timedBody{ReadCloser: res.Body, waited: waited}
		}
		return res, err
	})
}

// #endregion

// timedBody soma ao pedido o tempo de cada leitura do corpo: é o tempo
// de esperar o servidor mandar o resto.
type timedBody struct {
	io.ReadCloser
	waited *atomic.Int64
}

func (b *timedBody) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := b.ReadCloser.Read(p)
	b.waited.Add(int64(time.Since(start)))
	return n, err
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// #region metrics-admitter
// admitter é o porteiro de mcphttp, pelo método, sem importar o pacote.
type admitter interface {
	Admit(ctx context.Context, message []byte) (bool, []byte)
}

// Admitter conta as mensagens repassadas e as negadas pelo porteiro.
func (m *Metrics) Admitter(next admitter) admitter {
	return admitFunc(func(
		ctx context.Context, msg []byte,
	) (bool, []byte) {
		ok, reply := next.Admit(ctx, msg)
		if ok {
			m.forwarded.value.Add(1)
		} else {
			m.denied.value.Add(1)
		}
		return ok, reply
	})
}

// #endregion

type admitFunc func(context.Context, []byte) (bool, []byte)

func (f admitFunc) Admit(ctx context.Context, msg []byte) (bool, []byte) {
	return f(ctx, msg)
}

// #region metrics-serve
// ServeHTTP escreve as medidas no formato de texto do Prometheus.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	m.request.write(w)
	m.upstream.write(w)
	m.overhead.write(w)
	m.writeStages(w)
	writeCounters(w, "guarita_messages_total",
		"Mensagens do agente, repassadas ou negadas.",
		&m.forwarded, &m.denied)
	_, _ = fmt.Fprintf(w, "# HELP guarita_in_flight %s\n"+
		"# TYPE guarita_in_flight gauge\nguarita_in_flight %d\n",
		"Pedidos em curso.", m.inFlight.Load())
}

// #endregion

func writeCounters(w io.Writer, name, help string, cs ...*counter) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name,
		help, name)
	for _, c := range cs {
		_, _ = fmt.Fprintf(w, "%s{outcome=%q} %d\n", name, c.label,
			c.value.Load())
	}
}
