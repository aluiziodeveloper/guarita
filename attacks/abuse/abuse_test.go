package abuse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/breaker"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/adapters/quota"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// unitCost é o custo de ask_model no laboratório, em unidades.
const unitCost = 30

// paid é o servidor pago: conta cada chamada que chega e responde
// depois de delay, com o status dado.
func paid(t *testing.T, delay time.Duration, status int) (
	*url.URL, *atomic.Int64,
) {
	t.Helper()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			time.Sleep(delay)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL + "/mcp")
	return target, &calls
}

// guarita põe o servidor atrás da Guarita, com os inspetores e as
// opções do transporte dados.
func guarita(
	t *testing.T, target *url.URL, inspectors []app.Inspector,
	options ...mcphttp.Option,
) string {
	t.Helper()
	policy, _ := policystatic.New(map[string]domain.Decision{
		"ask_model": domain.DecisionAllow,
	})
	uc := app.NewInterceptCall(policy, nil, app.WithAgent(
		func(_ context.Context) string { return "relatorios" }),
		app.WithInspectors(inspectors...))
	handler, _ := mcphttp.NewHandler(target, mcpproxy.NewGate(uc),
		options...)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL + "/mcp"
}

// loop faz n chamadas seguidas a ask_model e devolve quanto levou.
func loop(t *testing.T, endpoint string, n int) time.Duration {
	t.Helper()
	start := time.Now()
	for id := range n {
		body := `{"jsonrpc":"2.0","id":` + strconv.Itoa(id) +
			`,"method":"tools/call","params":{"name":"ask_model"}}`
		r, err := http.Post(endpoint, "application/json",
			strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
	}
	return time.Since(start)
}

// #region attacks-abuse-loop-test
// O agente em laço faz 200 perguntas ao modelo pago. Sem limites, as
// 200 chegam ao servidor, e a fatura é de 6000 unidades. Com o
// orçamento de 300 unidades por hora, chegam 10.
func TestLaco(t *testing.T) {
	limits := quota.New(quota.Limits{
		Cost:   map[string]int64{"ask_model": unitCost},
		Budget: &quota.Rate{N: 300, Per: time.Hour, Burst: 300},
	})
	for _, c := range []struct {
		name       string
		inspectors []app.Inspector
		want       int64
	}{
		{"sem limites", nil, 200}, {"com orçamento", []app.Inspector{
			limits,
		}, 10},
	} {
		target, calls := paid(t, 0, http.StatusOK)
		loop(t, guarita(t, target, c.inspectors), 200)
		t.Logf("%-13s chegaram %3d; fatura de %4d unidades", c.name,
			calls.Load(), calls.Load()*unitCost)
		if calls.Load() != c.want {
			t.Errorf("%s: chegaram %d", c.name, calls.Load())
		}
	}
}

// #endregion

// #region attacks-abuse-breaker-test
// O servidor demora 200 ms e responde 502. Sem disjuntor, as 20
// chamadas do laço chegam a ele, e o agente espera todas. Com o
// disjuntor de três falhas, chegam três, e o resto volta na hora.
func TestServidorQueFalha(t *testing.T) {
	for _, c := range []struct {
		name string
		trip bool
		want int64
	}{{"sem disjuntor", false, 20}, {"com disjuntor", true, 3}} {
		target, calls := paid(t, 200*time.Millisecond,
			http.StatusBadGateway)
		var options []mcphttp.Option
		if c.trip {
			options = append(options, mcphttp.WithTransport(breaker.New(
				http.DefaultTransport, 3, time.Minute, nil)))
		}
		took := loop(t, guarita(t, target, nil, options...), 20)
		t.Logf("%-13s chegaram %2d; o laço levou %v", c.name,
			calls.Load(), took.Round(10*time.Millisecond))
		if calls.Load() != c.want {
			t.Errorf("%s: chegaram %d", c.name, calls.Load())
		}
	}
}

// #endregion
