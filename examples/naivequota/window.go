// Package naivequota mostra a cota ingênua do Capítulo 18: um contador
// por minuto do relógio, zerado quando o minuto vira.
package naivequota

import (
	"sync"
	"time"
)

// #region examples-naivequota
// Window conta as chamadas do minuto corrente e recusa a partir de
// limit. O contador zera quando o minuto do relógio muda, e é aí que
// a cota falha: limit chamadas no fim de um minuto e limit no começo
// do seguinte passam em poucos segundos.
type Window struct {
	mu     sync.Mutex
	limit  int
	minute int64
	count  int
}

// NewWindow cria a janela com limit chamadas por minuto.
func NewWindow(limit int) *Window { return &Window{limit: limit} }

// Allow conta a chamada feita em now e diz se ela cabe na cota.
func (w *Window) Allow(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if minute := now.Unix() / 60; minute != w.minute {
		w.minute, w.count = minute, 0
	}
	w.count++
	return w.count <= w.limit
}

// #endregion
