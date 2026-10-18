package mcphttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/breaker"
)

// #region mcphttp-breaker-test
// Um servidor que responde 502 a tudo, atrás do disjuntor: as cinco
// primeiras chamadas chegam a ele e voltam com o 502 dele; as
// seguintes voltam na hora com 503 e Retry-After, sem chegar.
func TestHandler_disjuntor(t *testing.T) {
	var reached atomic.Int32
	failing := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			reached.Add(1)
			w.WriteHeader(http.StatusBadGateway)
		}))
	t.Cleanup(failing.Close)
	target, _ := url.Parse(failing.URL + "/mcp")
	tripper := breaker.New(http.DefaultTransport, 5, 30*time.Second,
		nil)
	handler, _ := NewHandler(target, allowOnly(t, "read_file"),
		WithTransport(tripper))
	guarita := httptest.NewServer(handler)
	t.Cleanup(guarita.Close)
	for n := range 8 {
		r, _ := post(t, guarita.URL+"/mcp", toolCall(n, "read_file"))
		t.Logf("chamada %d: %d Retry-After=%q", n+1, r.StatusCode,
			r.Header.Get("Retry-After"))
	}
	if reached.Load() != 5 || tripper.State() != breaker.Open {
		t.Errorf("chegaram %d; estado %s", reached.Load(),
			tripper.State())
	}
}

// #endregion
