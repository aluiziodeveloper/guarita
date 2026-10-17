// Package mcphttp serve o MCP sobre HTTP (Streamable HTTP) na frente de
// um servidor remoto, com a mesma porta do transporte stdio.
package mcphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
)

// maxBodyBytes limita o corpo de um POST, como o limite de mensagem do
// transporte stdio.
const maxBodyBytes = 4 << 20

// Prazos padrão de cada POST.
const (
	DefaultCallTimeout = 30 * time.Second
	DefaultBodyTimeout = 10 * time.Second
)

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
	proxy       *httputil.ReverseProxy
	callTimeout time.Duration
	bodyTimeout time.Duration
}

// #region mcphttp-options
// Option ajusta um Handler: é uma opção funcional.
type Option func(*Handler)

// WithCallTimeout limita cada chamada, da leitura do pedido ao fim da
// resposta, inclusive quando ela chega em fluxo.
func WithCallTimeout(d time.Duration) Option {
	return func(h *Handler) { h.callTimeout = d }
}

// WithBodyTimeout limita o tempo para o cliente enviar o corpo.
func WithBodyTimeout(d time.Duration) Option {
	return func(h *Handler) { h.bodyTimeout = d }
}

// #endregion

// #region mcphttp-new-handler
// NewHandler monta o proxy reverso para upstream. Rewrite troca o
// destino e mantém o caminho do servidor, qualquer que seja o recebido.
func NewHandler(
	upstream *url.URL, gate Admitter, options ...Option,
) (*Handler, error) {
	if upstream == nil || upstream.Host == "" ||
		(upstream.Scheme != "http" && upstream.Scheme != "https") {
		return nil, ErrUpstream
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.URL.Path = upstream.Path
			r.Out.URL.RawPath = upstream.RawPath
		},
		ErrorHandler: proxyError,
	}
	handler := &Handler{
		gate: gate, proxy: proxy,
		callTimeout: DefaultCallTimeout,
		bodyTimeout: DefaultBodyTimeout,
	}
	for _, option := range options {
		option(handler)
	}
	return handler, nil
}

// #endregion

// #region mcphttp-serve
// ServeHTTP atende os três métodos do endpoint MCP. Só o POST leva
// mensagens do agente; GET e DELETE abrem e encerram o fluxo da sessão.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), h.callTimeout)
	defer cancel()
	body, status, err := h.readBody(w, r)
	if err != nil {
		writeError(w, status, codeInvalidRequest, err.Error())
		return
	}
	if forward, reply := h.gate.Admit(ctx, body); !forward {
		writeJSON(w, replyStatus(reply), reply)
		return
	}
	message, _ := mcpproxy.ParseMessage(body) // a porta já o aceitou
	if err := checkMirrored(r.Header, message); err != nil {
		writeErrorID(w, http.StatusBadRequest, message.ID,
			codeHeaderMismatch, err.Error())
		return
	}
	out := r.Clone(ctx) // o pedido recebido não é alterado
	out.Body = io.NopCloser(bytes.NewReader(body))
	out.ContentLength = int64(len(body))
	h.proxy.ServeHTTP(w, out)
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
