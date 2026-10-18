// Package mcphttp serve o MCP sobre HTTP (Streamable HTTP) na frente de
// um servidor remoto, com a mesma porta do transporte stdio.
package mcphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

// maxBodyBytes limita o corpo de um POST, como o limite de mensagem do
// transporte stdio.
const maxBodyBytes = 4 << 20

// Prazos padrão de cada POST.
const (
	DefaultCallTimeout = 30 * time.Second
	DefaultBodyTimeout = 10 * time.Second
)

// methodSubscriptionsListen abre o fluxo de notificações da revisão
// 2026-07-28, no lugar do GET das revisões anteriores.
const methodSubscriptionsListen = "subscriptions/listen"

// Códigos JSON-RPC das respostas que a própria Guarita produz.
const (
	codeInvalidRequest = -32600
	codeInternalError  = -32603
)

// ErrUpstream recusa um endereço de servidor que não seja http ou https.
var ErrUpstream = errors.New("upstream must be an http or https URL")

var (
	errSlowBody = errors.New("request body too slow")
	errReadBody = errors.New("cannot read request body")
)

// #region mcphttp-admitter
// Admitter é a porta do Capítulo 6 vista daqui: decide sobre uma
// mensagem e, quando nega, devolve a resposta de erro.
type Admitter interface {
	Admit(ctx context.Context, message []byte) (bool, []byte)
}

// #endregion

var _ Admitter = (*mcpproxy.Gate)(nil)

// Handler é o endpoint MCP da Guarita: inspeciona o que chega por POST
// e repassa ao servidor pelo proxy reverso.
type Handler struct {
	gate        Admitter
	upstream    *url.URL
	proxy       *httputil.ReverseProxy
	callTimeout time.Duration
	bodyTimeout time.Duration
	credential  secret.Secret
	logger      *slog.Logger
	agent       func(context.Context) string
	catalog     ToolReviewer
	results     ResultReviewer
}

// #region mcphttp-options
// Option ajusta um Handler: é uma opção funcional.
type Option func(*Handler)

// WithCallTimeout limita cada chamada, da leitura do pedido ao fim da
// resposta, inclusive quando ela chega em fluxo.
func WithCallTimeout(d time.Duration) Option {
	return func(h *Handler) { h.callTimeout = d }
}

// WithUpstreamCredential faz a Guarita se apresentar ao servidor com a
// própria credencial, no lugar de qualquer uma que o agente mande.
func WithUpstreamCredential(credential secret.Secret) Option {
	return func(h *Handler) { h.credential = credential }
}

// WithLogger registra cada pedido, e os erros do proxy, em logger.
func WithLogger(logger *slog.Logger) Option {
	return func(h *Handler) { h.logger = logger }
}

// WithAgent diz ao registro como saber quem é o agente do pedido.
func WithAgent(agent func(context.Context) string) Option {
	return func(h *Handler) { h.agent = agent }
}

// WithBodyTimeout limita o tempo para o cliente enviar o corpo.
func WithBodyTimeout(d time.Duration) Option {
	return func(h *Handler) { h.bodyTimeout = d }
}

// #endregion

// #region mcphttp-new-handler
// NewHandler monta o proxy reverso para upstream. As opções são
// aplicadas antes, porque rewrite lê o que elas configuram.
func NewHandler(
	upstream *url.URL, gate Admitter, options ...Option,
) (*Handler, error) {
	if upstream == nil || upstream.Host == "" ||
		(upstream.Scheme != "http" && upstream.Scheme != "https") {
		return nil, ErrUpstream
	}
	handler := &Handler{
		gate: gate, upstream: upstream,
		callTimeout: DefaultCallTimeout,
		bodyTimeout: DefaultBodyTimeout,
		logger:      slog.New(slog.DiscardHandler),
		agent:       func(context.Context) string { return "" },
	}
	for _, option := range options {
		option(handler)
	}
	handler.proxy = &httputil.ReverseProxy{
		Rewrite: handler.rewrite, ErrorHandler: proxyError,
		ModifyResponse: handler.modifyResponse,
		ErrorLog: slog.NewLogLogger(handler.logger.Handler(),
			slog.LevelWarn),
	}
	return handler, nil
}

// #endregion

// #region mcphttp-rewrite
// rewrite monta o pedido ao servidor: o destino e o caminho dele, só os
// cabeçalhos permitidos e, se houver, a credencial da própria Guarita.
func (h *Handler) rewrite(r *httputil.ProxyRequest) {
	r.SetURL(h.upstream)
	r.Out.URL.Path = h.upstream.Path
	r.Out.URL.RawPath = h.upstream.RawPath
	r.Out.Header = allowedHeaders(r.Out.Header)
	if !h.credential.IsZero() {
		r.Out.Header.Set("Authorization",
			"Bearer "+h.credential.Reveal())
	}
}

// #endregion

// #region mcphttp-serve
// ServeHTTP atende os três métodos do endpoint MCP. Só o POST leva
// mensagens do agente; GET e DELETE abrem e encerram o fluxo da sessão.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w := &accessWriter{ResponseWriter: rw, status: http.StatusOK}
	defer h.logAccess(r, w, time.Now())
	switch r.Method {
	case http.MethodPost:
		h.post(w, r)
	case http.MethodGet, http.MethodDelete:
		h.proxy.ServeHTTP(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeError(w, http.StatusMethodNotAllowed, codeInvalidRequest,
			"method not allowed")
	}
}

// #endregion

// #region mcphttp-post
// post lê o corpo, dentro dos limites, e só o repassa se a porta
// deixar. O servidor recebe os mesmos bytes que a porta leu.
func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	body, status, err := h.readBody(w, r)
	if err != nil {
		writeError(w, status, codeInvalidRequest, err.Error())
		return
	}
	if forward, reply := h.gate.Admit(r.Context(), body); !forward {
		writeJSON(w, replyStatus(reply), reply)
		return
	}
	message, _ := mcpproxy.ParseMessage(body) // a porta já o aceitou
	noteMessage(w, message)
	if err := checkMirrored(r.Header, message); err != nil {
		writeErrorID(w, http.StatusBadRequest, message.ID,
			codeHeaderMismatch, err.Error())
		return
	}
	ctx, cancel := h.callContext(r.Context(), message.Method)
	defer cancel()
	ctx = h.markRequest(ctx, message)
	out := r.Clone(ctx) // o pedido recebido não é alterado
	out.Body = io.NopCloser(bytes.NewReader(body))
	out.ContentLength = int64(len(body))
	h.proxy.ServeHTTP(w, out)
}

// #endregion

// #region mcphttp-call-context
// callContext dá prazo a cada chamada, menos a subscriptions/listen da
// revisão 2026-07-28: esse fluxo fica aberto enquanto o agente quiser.
func (h *Handler) callContext(
	ctx context.Context, method string,
) (context.Context, context.CancelFunc) {
	if method == methodSubscriptionsListen {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, h.callTimeout)
}

// #endregion

// #region mcphttp-read-body
// readBody aplica dois limites: tamanho, com MaxBytesReader, e tempo,
// com o prazo de leitura da conexão, contra o cliente lento.
func (h *Handler) readBody(
	w http.ResponseWriter, r *http.Request,
) ([]byte, int, error) {
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(h.bodyTimeout))
	limited := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return nil, http.StatusRequestEntityTooLarge,
			mcpproxy.ErrMessageTooLarge
	case errors.Is(err, os.ErrDeadlineExceeded):
		return nil, http.StatusRequestTimeout, errSlowBody
	case err != nil:
		return nil, http.StatusBadRequest, errReadBody
	}
	// Só aqui o prazo sai: com erro, ele precisa valer também para o
	// descarte do resto do corpo que o servidor faz depois da resposta.
	_ = controller.SetReadDeadline(time.Time{})
	return body, http.StatusOK, nil
}

// #endregion

// replyStatus escolhe o código HTTP da recusa: sem id, a mensagem nem
// foi lida (400); com id, é uma resposta JSON-RPC de erro válida (200).
func replyStatus(reply []byte) int {
	var head struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(reply, &head) != nil || len(head.ID) == 0 ||
		string(head.ID) == "null" {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

// #region mcphttp-proxy-error
// proxyError traduz a falha do repasse. Prazo esgotado é 504; servidor
// fora do ar é 502; o agente que desistiu não recebe nada.
func proxyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(r.Context().Err(), context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, codeInternalError,
			"upstream timeout")
	case errors.Is(err, errUpstreamResponse),
		errors.Is(err, errCatalog):
		writeError(w, http.StatusBadGateway, codeInternalError,
			err.Error())
	default:
		writeError(w, http.StatusBadGateway, codeInternalError,
			"upstream unavailable")
	}
}

// #endregion

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      any      `json:"id"`
	Error   rpcError `json:"error"`
}

// writeError responde com um erro JSON-RPC sem id, como a especificação
// prevê para o que a Guarita recusa antes de ler a mensagem.
func writeError(w http.ResponseWriter, status, code int, text string) {
	writeErrorID(w, status, nil, code, text)
}

// writeErrorID responde com o erro ligado ao id do pedido, quando há um.
func writeErrorID(
	w http.ResponseWriter, status int, id json.RawMessage, code int,
	text string,
) {
	response := errorResponse{JSONRPC: "2.0", Error: rpcError{code, text}}
	if len(id) > 0 {
		response.ID = id
	}
	body, _ := json.Marshal(response)
	writeJSON(w, status, append(body, '\n'))
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
