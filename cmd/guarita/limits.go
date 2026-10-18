package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/breaker"
	"github.com/aluiziodeveloper/guarita/internal/adapters/quota"
	"github.com/aluiziodeveloper/guarita/internal/app"
)

var errLimits = errors.New("invalid limit")

// limitsConfig são as cotas e o disjuntor do Capítulo 18.
type limitsConfig struct {
	calls    map[string]quota.Rate // -quota, por ferramenta
	cost     map[string]int64      // -cost, unidades por chamada
	budget   *quota.Rate           // -budget, unidades por agente
	failures int                   // -breaker-failures; 0 desliga
	cooldown time.Duration
}

// #region cmd-guarita-limits-flags
// limitsFlags declara as cotas, repetíveis por ferramenta, o orçamento
// de custo de cada agente e o disjuntor de cada servidor.
func limitsFlags(flags *flag.FlagSet, c *limitsConfig) {
	flags.Func("quota", "chamadas por agente: ferramenta=60/m[:rajada]",
		pair(func(k, v string) error {
			r, err := quota.ParseRate(v)
			c.calls = set(c.calls, k, r)
			return err
		}))
	flags.Func("cost", "unidades de cada chamada: ferramenta=30",
		pair(func(k, v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("%w: -cost %s=%s", errLimits, k, v)
			}
			c.cost = set(c.cost, k, n)
			return nil
		}))
	flags.Func("budget", "unidades por agente: 1000/h[:rajada]",
		func(v string) error {
			r, err := quota.ParseRate(v)
			c.budget = &r
			return err
		})
	breakerFlags(flags, c)
}

// #endregion

// breakerFlags declara o disjuntor; zero falhas o desliga.
func breakerFlags(flags *flag.FlagSet, c *limitsConfig) {
	flags.IntVar(&c.failures, "breaker-failures", 0,
		"falhas seguidas que abrem o disjuntor; 0 desliga")
	flags.DurationVar(&c.cooldown, "breaker-cooldown", 30*time.Second,
		"tempo do disjuntor aberto antes do teste")
}

// pair lê uma opção nome=valor.
func pair(apply func(key, value string) error) func(string) error {
	return func(text string) error {
		k, v, ok := strings.Cut(text, "=")
		if !ok || k == "" {
			return fmt.Errorf("%w: %q is not name=value", errLimits, text)
		}
		return apply(k, v)
	}
}

// set grava no mapa, criando-o na primeira vez.
func set[V any](m map[string]V, k string, v V) map[string]V {
	if m == nil {
		m = map[string]V{}
	}
	m[k] = v
	return m
}

// #region cmd-guarita-new-quota
// newQuota cria o inspetor de cotas, um só para todas as rotas: o
// orçamento é do agente, e não de uma rota. Um custo maior que a
// rajada do orçamento nunca caberia, e impede a partida. O retorno é a
// interface, e não *quota.Inspector: um ponteiro nulo dentro de uma
// interface não é uma interface nula.
func (c limitsConfig) newQuota() (app.Inspector, error) {
	if len(c.calls) == 0 && c.budget == nil {
		return nil, nil
	}
	for tool, cost := range c.cost {
		if c.budget != nil && cost > c.budget.Burst {
			return nil, fmt.Errorf("%w: -cost %s=%d over the burst %d",
				errLimits, tool, cost, c.budget.Burst)
		}
	}
	return quota.New(quota.Limits{
		Calls: c.calls, Cost: c.cost, Budget: c.budget,
	}), nil
}

// #endregion

// #region cmd-guarita-breaker
// transport é o disjuntor do servidor da rota, com cada mudança de
// estado no registro. Sem -breaker-failures, o repasse é o de sempre.
func (c limitsConfig) transport(
	server string, logger *slog.Logger,
) http.RoundTripper {
	if c.failures <= 0 {
		return nil
	}
	return breaker.New(http.DefaultTransport, c.failures, c.cooldown,
		func(from, to breaker.State) {
			logger.Warn("disjuntor", "server", server,
				"from", string(from), "to", string(to))
		})
}

// #endregion
