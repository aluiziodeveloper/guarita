// Package naiveapproval mostra a aprovação ingênua do Capítulo 12: uma
// fila sem limite, com números sequenciais, aprovada por um endereço
// que fica no mesmo servidor que o agente usa e não pede credencial.
package naiveapproval

import (
	"net/http"
	"strconv"
	"sync"
)

// #region examples-naiveapproval
// Queue numera os pedidos de 1 em diante e guarda os aprovados.
type Queue struct {
	mu       sync.Mutex
	last     int
	approved map[int]bool
}

// Request põe um pedido na fila e devolve o número dele.
func (q *Queue) Request() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.last++
	return q.last
}

// ServeHTTP aprova o pedido de ?id=N. Quem chama não é perguntado.
func (q *Queue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.approved == nil {
		q.approved = map[int]bool{}
	}
	q.approved[id] = true
}

// #endregion

// Approved diz se o pedido foi aprovado.
func (q *Queue) Approved(id int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.approved[id]
}

// Pending conta os pedidos ainda sem aprovação.
func (q *Queue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.last - len(q.approved)
}
