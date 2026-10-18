// Package breaker é o disjuntor do repasse HTTP: depois de falhas
// seguidas do servidor, ele para de chamá-lo por um tempo e recusa na
// hora, em vez de deixar cada chamada do agente esperar o prazo.
package breaker

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ErrOpen indica que o disjuntor está aberto.
var ErrOpen = errors.New("breaker: circuit open")

// State é o estado do disjuntor.
type State string

// Os três estados.
const (
	Closed   State = "fechado"     // as chamadas passam
	Open     State = "aberto"      // recusa na hora
	HalfOpen State = "meio-aberto" // uma chamada de teste passa
)

// #region breaker-transitions
// event é o que acontece com o disjuntor.
type event string

const (
	success  event = "sucesso"
	tripped  event = "falhas no limite"
	cooled   event = "resfriamento cumprido"
	probeBad event = "teste falhou"
)

// transitions é a tabela de estados, como a das aprovações do
// Capítulo 12: o que não está nela não muda o estado.
var transitions = map[State]map[event]State{
	Closed:   {tripped: Open},
	Open:     {cooled: HalfOpen},
	HalfOpen: {success: Closed, probeBad: Open},
}

// #endregion

// OpenError é a recusa do disjuntor aberto, com quanto falta para o
// próximo teste. Ela é ErrOpen para errors.Is.
type OpenError struct{ Retry time.Duration }

func (e OpenError) Error() string        { return ErrOpen.Error() }
func (e OpenError) Is(target error) bool { return target == ErrOpen }

// RetryAfter diz quanto esperar; o transporte HTTP o lê por uma
// interface própria, sem importar este pacote.
func (e OpenError) RetryAfter() time.Duration { return e.Retry }

// #region breaker-type
// Breaker envolve o RoundTripper do repasse. A trava protege um estado
// de vários campos que mudam juntos, o que um atomic sozinho não faz.
type Breaker struct {
	base      http.RoundTripper
	threshold int
	cooldown  time.Duration
	onChange  func(from, to State)

	mu       sync.Mutex
	state    State
	failures int
	openedAt time.Time
	probing  bool
}

// New cria o disjuntor fechado, que abre depois de threshold falhas
// seguidas. onChange roda com a trava tomada e não chama o disjuntor.
func New(
	base http.RoundTripper, threshold int, cooldown time.Duration,
	onChange func(from, to State),
) *Breaker {
	if onChange == nil {
		onChange = func(State, State) {}
	}
	return &Breaker{
		base: base, threshold: threshold, cooldown: cooldown,
		onChange: onChange, state: Closed,
	}
}

// #endregion

// #region breaker-round-trip
// RoundTrip recusa na hora com o disjuntor aberto; senão, repassa e
// conta o resultado. Um pedido cancelado pelo agente não conta: não
// diz nada sobre o servidor.
func (b *Breaker) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := b.admit(); err != nil {
		return nil, err
	}
	response, err := b.base.RoundTrip(r)
	if errors.Is(err, context.Canceled) {
		b.release()
		return response, err
	}
	b.record(err != nil || response.StatusCode >= 500)
	return response, err
}

// #endregion

// #region breaker-admit
// admit decide se o pedido sai. Aberto, ele só sai depois do
// resfriamento, como o único teste; meio-aberto com um teste em curso,
// os outros esperam o resultado dele recusados.
func (b *Breaker) admit() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == Open {
		left := b.cooldown - time.Since(b.openedAt)
		if left > 0 {
			return OpenError{Retry: left}
		}
		b.move(cooled)
	}
	if b.state == HalfOpen {
		if b.probing {
			return OpenError{Retry: b.cooldown}
		}
		b.probing = true
	}
	return nil
}

// #endregion

// #region breaker-record
// record conta o resultado. Fechado, as falhas seguidas abrem o
// disjuntor no limite, e um sucesso zera a conta; meio-aberto, o teste
// decide.
func (b *Breaker) record(failed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.state == HalfOpen && failed:
		b.move(probeBad)
	case b.state == HalfOpen:
		b.move(success)
	case failed:
		if b.failures++; b.failures >= b.threshold {
			b.move(tripped)
		}
	default:
		b.failures = 0
	}
}

// #endregion

// release devolve o teste de um pedido cancelado sem resultado.
func (b *Breaker) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
}

// move aplica a transição, se a tabela tiver uma, com a trava tomada.
func (b *Breaker) move(e event) {
	to, ok := transitions[b.state][e]
	if !ok {
		return
	}
	from := b.state
	b.state, b.failures, b.probing = to, 0, false
	if to == Open {
		b.openedAt = time.Now()
	}
	b.onChange(from, to)
}

// State devolve o estado atual.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
