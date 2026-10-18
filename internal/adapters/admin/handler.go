// Package admin é a interface do operador: a lista de pedidos de
// aprovação e a decisão sobre cada um, por API e por uma página web.
// Ela fica num endereço próprio, longe do endpoint do agente.
package admin

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/approval"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

//go:embed page.html
var pages embed.FS

var page = template.Must(template.ParseFS(pages, "page.html"))

// Approvals é o que a interface usa da fila.
type Approvals interface {
	Pending() []approval.Request
	Decide(id, operator string, approve bool) error
}

// Item é um pedido como a API o mostra.
type Item struct {
	ID        string          `json:"id"`
	Agent     string          `json:"agent"`
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Deadline  time.Time       `json:"deadline"`
}

// #region admin-handler
// NewHandler monta as rotas do operador. Toda rota exige um operador;
// a proteção contra pedidos de outra origem fica por fora, em quem
// compõe, junto com a guarda de Host.
func NewHandler(queue Approvals, operators Operators) http.Handler {
	h := &handler{queue: queue}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.page)
	mux.HandleFunc("GET /approvals", h.list)
	mux.HandleFunc("POST /approvals/{id}/approve", h.decide(true))
	mux.HandleFunc("POST /approvals/{id}/deny", h.decide(false))
	return requireOperator(operators, mux)
}

// #endregion

// requireOperator responde 401 a quem não é operador e põe o nome do
// operador no contexto dos outros.
func requireOperator(ops Operators, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operator, ok := ops.Authenticate(r)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="guarita"`)
			http.Error(w, "operator required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withOperator(r.Context(),
			operator)))
	})
}

type handler struct{ queue Approvals }

// #region admin-decide
// decide aplica a decisão do operador e traduz os erros da fila em
// estados HTTP. O formulário da página volta para ela com 303.
func (h *handler) decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h.queue.Decide(r.PathValue("id"),
			operatorFrom(r.Context()), approve)
		switch {
		case errors.Is(err, approval.ErrUnknown):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, approval.ErrSelfApproval):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, domain.ErrInvalidTransition):
			http.Error(w, err.Error(), http.StatusConflict)
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case r.FormValue("from") == "page":
			http.Redirect(w, r, "/", http.StatusSeeOther)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// #endregion

func (h *handler) list(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items(h.queue.Pending()))
}

// #region admin-page
// page desenha a fila com html/template: os argumentos vêm do agente e
// podem trazer marcação; o template os escapa conforme o contexto em
// que aparecem.
func (h *handler) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, struct {
		Operator string
		Items    []Item
	}{operatorFrom(r.Context()), items(h.queue.Pending())})
}

// #endregion

func items(requests []approval.Request) []Item {
	out := make([]Item, 0, len(requests))
	for _, r := range requests {
		arguments := r.Input.Call.Arguments
		if len(arguments) == 0 {
			arguments = json.RawMessage("{}")
		}
		out = append(out, Item{
			ID: r.ID, Agent: r.Input.Agent, Server: r.Input.Server,
			Tool: r.Input.Call.ToolName, Arguments: arguments,
			Deadline: r.Deadline,
		})
	}
	return out
}
