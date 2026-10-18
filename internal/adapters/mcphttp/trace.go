package mcphttp

import (
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// propagator lê e escreve o cabeçalho traceparent do W3C Trace Context.
var propagator = propagation.TraceContext{}

// WithTracer faz a Guarita abrir um span a cada POST. Sem ele, o
// tracer é o nulo: a Guarita só carrega adiante o traceparent que
// recebeu, sem criar spans próprios.
func WithTracer(tracer trace.Tracer) Option {
	return func(h *Handler) { h.tracer = tracer }
}

// nopTracer é o tracer padrão, que não registra nada.
var nopTracer = noop.NewTracerProvider().Tracer("guarita")

// #region mcphttp-trace
// trace extrai o contexto de trace do pedido do agente, se houver, e
// abre o span da Guarita como filho dele. O contexto devolvido vai com
// o pedido até a porta, que o passa à trilha de auditoria, e até o
// repasse, que o injeta no pedido ao servidor.
func (h *Handler) trace(r *http.Request) (*http.Request, func()) {
	ctx := propagator.Extract(r.Context(),
		propagation.HeaderCarrier(r.Header))
	ctx, span := h.tracer.Start(ctx, "guarita.mcp",
		trace.WithSpanKind(trace.SpanKindServer))
	return r.WithContext(ctx), func() { span.End() }
}

// #endregion

// #region mcphttp-inject
// inject escreve o traceparent do span da Guarita no pedido ao
// servidor, depois do filtro de cabeçalhos: o do agente, se havia um,
// é substituído pelo da Guarita, que é filho dele no mesmo trace.
func inject(r *http.Request, out http.Header) {
	propagator.Inject(r.Context(), propagation.HeaderCarrier(out))
}

// #endregion
