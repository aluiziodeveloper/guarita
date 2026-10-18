package breaker

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

// upstream é um servidor falso: responde o status da vez, sem rede.
type upstream struct {
	status int
	calls  int
}

func (u *upstream) RoundTrip(*http.Request) (*http.Response, error) {
	u.calls++
	return &http.Response{StatusCode: u.status, Body: http.NoBody}, nil
}

func get(t *testing.T, b *Breaker) error {
	t.Helper()
	r, _ := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"http://lab.test/mcp", nil)
	response, err := b.RoundTrip(r)
	if response != nil {
		_ = response.Body.Close()
	}
	return err
}

// recorder anota cada transição como "de→para".
func recorder() (*[]string, func(from, to State)) {
	var changes []string
	return &changes, func(from, to State) {
		changes = append(changes, string(from)+"→"+string(to))
	}
}

// #region breaker-test
// Cinco falhas seguidas abrem o disjuntor, e as chamadas seguintes nem
// chegam ao servidor. Passado o resfriamento, um teste passa: se o
// servidor voltou, o disjuntor fecha; se não, abre de novo.
func TestBreaker_ciclo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u := &upstream{status: http.StatusBadGateway}
		changes, onChange := recorder()
		b := New(u, 5, 30*time.Second, onChange)
		for range 8 {
			_ = get(t, b)
		}
		t.Logf("8 chamadas, servidor em 502: %d chegaram; %s", u.calls,
			b.State())
		var open OpenError
		if err := get(t, b); !errors.As(err, &open) || u.calls != 5 {
			t.Fatalf("aberto: %v, %d chamadas", err, u.calls)
		}
		t.Logf("recusa: %v, tentar em %v", open, open.RetryAfter())
		time.Sleep(30 * time.Second)
		_ = get(t, b) // o teste falha: abre de novo
		time.Sleep(30 * time.Second)
		u.status = http.StatusOK
		_ = get(t, b) // o teste passa: fecha
		t.Logf("transições: %v", *changes)
		if b.State() != Closed || len(*changes) != 5 {
			t.Errorf("estado %s, %v", b.State(), *changes)
		}
	})
}

// #endregion

func TestBreaker_sucessoZeraAConta(t *testing.T) {
	u := &upstream{status: http.StatusBadGateway}
	b := New(u, 3, time.Minute, nil)
	_, _ = get(t, b), get(t, b)
	u.status = http.StatusOK
	_ = get(t, b)
	u.status = http.StatusBadGateway
	_, _ = get(t, b), get(t, b)
	if b.State() != Closed {
		t.Errorf("falhas não seguidas abriram o disjuntor")
	}
}
