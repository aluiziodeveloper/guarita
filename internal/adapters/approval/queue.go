// Package approval guarda os pedidos de aprovação humana em memória:
// a chamada do agente espera aqui até um operador decidir ou o prazo
// acabar.
package approval

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var (
	// ErrUnknown indica um identificador que não está na fila.
	ErrUnknown = errors.New("no such approval request")
	// ErrSelfApproval recusa o agente decidindo o próprio pedido.
	ErrSelfApproval = errors.New("agent cannot decide its own request")
	// ErrNoOperator recusa uma decisão sem operador identificado.
	ErrNoOperator = errors.New("decision needs an operator")
)

var _ app.Approver = (*Queue)(nil)

// Request é um pedido como o operador o vê.
type Request struct {
	ID        string
	Input     domain.PolicyInput
	State     domain.ApprovalState
	Created   time.Time
	Deadline  time.Time
	DecidedBy string
}

// entry é um pedido e o sinal de que ele saiu de pending: o canal é
// fechado uma vez, e todo receptor acorda.
type entry struct {
	request Request
	done    chan struct{}
	key     string // a chamada, para a repetição depois de negada
}

// Queue é a fila. O mutex protege o mapa e o estado de cada pedido;
// a espera acontece fora dele.
type Queue struct {
	mu         sync.Mutex
	entries    map[string]*entry
	timeout    time.Duration
	maxPending int                  // por agente; 0 é sem limite
	cooldown   time.Duration        // 0 desliga
	denied     map[string]time.Time // chave da chamada -> até quando
}

// New cria a fila com o prazo de cada pedido.
func New(timeout time.Duration, options ...Option) *Queue {
	q := &Queue{
		entries: map[string]*entry{}, timeout: timeout,
		denied: map[string]time.Time{},
	}
	for _, option := range options {
		option(q)
	}
	return q
}

// #region approval-await
// Await põe a chamada na fila e espera o primeiro de três sinais: a
// decisão do operador, o fim do prazo ou a desistência do agente.
func (q *Queue) Await(
	ctx context.Context, input domain.PolicyInput,
) (domain.ApprovalState, error) {
	e, err := q.add(input)
	if err != nil {
		return "", err
	}
	timer := time.NewTimer(q.timeout)
	defer timer.Stop()
	select {
	case <-e.done: // o operador decidiu
	case <-timer.C: // se o operador decidiu no mesmo instante, vale ele
		_ = q.finish(e, domain.ApprovalExpired, "")
	case <-ctx.Done():
		_ = q.finish(e, domain.ApprovalCanceled, "")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return e.request.State, nil
}

// #endregion

// #region approval-add
// add registra o pedido com um identificador aleatório: rand.Text dá
// 26 caracteres de base32, mais de 128 bits, que ninguém adivinha.
func (q *Queue) add(input domain.PolicyInput) (*entry, error) {
	key := callKey(input)
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.admit(input, key); err != nil {
		return nil, err
	}
	now := time.Now()
	e := &entry{done: make(chan struct{}), key: key, request: Request{
		ID: rand.Text(), Input: input, State: domain.ApprovalPending,
		Created: now, Deadline: now.Add(q.timeout),
	}}
	q.entries[e.request.ID] = e
	return e, nil
}

// #endregion

// #region approval-finish
// finish muda o estado se a tabela do domínio deixar e acorda quem
// espera. Se o operador e o prazo chegarem juntos, o primeiro a pegar o
// mutex ganha; o segundo recebe ErrInvalidTransition.
func (q *Queue) finish(
	e *entry, to domain.ApprovalState, by string,
) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	next, err := e.request.State.Next(to)
	if err != nil {
		return err
	}
	e.request.State, e.request.DecidedBy = next, by
	if next == domain.ApprovalDenied && q.cooldown > 0 {
		q.denied[e.key] = time.Now().Add(q.cooldown)
	}
	close(e.done)
	return nil
}

// #endregion

// #region approval-decide
// Decide registra a decisão de um operador. O agente que pediu não
// decide o próprio pedido, e um pedido já decidido não muda.
func (q *Queue) Decide(id, operator string, approve bool) error {
	q.mu.Lock()
	e, ok := q.entries[id]
	q.mu.Unlock()
	switch {
	case !ok:
		return ErrUnknown
	case operator == "":
		return ErrNoOperator
	case operator == e.request.Input.Agent:
		return ErrSelfApproval
	}
	to := domain.ApprovalDenied
	if approve {
		to = domain.ApprovalApproved
	}
	return q.finish(e, to, operator)
}

// #endregion

// Pending devolve os pedidos que esperam, do mais antigo ao mais novo.
func (q *Queue) Pending() []Request {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []Request
	for _, e := range q.entries {
		if e.request.State == domain.ApprovalPending {
			out = append(out, e.request)
		}
	}
	slices.SortFunc(out, func(a, b Request) int {
		return a.Created.Compare(b.Created)
	})
	return out
}
