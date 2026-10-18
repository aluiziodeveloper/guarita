package quota

import (
	"testing"
	"testing/synctest"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// count faz n chamadas seguidas e devolve quantas passaram e os
// achados da última recusa.
func count(
	t *testing.T, i *Inspector, agent, tool string, n int,
) (int, []domain.Finding) {
	t.Helper()
	allowed, last := 0, []domain.Finding(nil)
	for range n {
		got, _ := i.Inspect(t.Context(), domain.PolicyInput{
			Agent: agent, Server: "llm", Call: domain.Call{ToolName: tool},
		})
		if got.Decision == domain.DecisionAllow {
			allowed++
		} else {
			last = got.Findings
		}
	}
	return allowed, last
}

// #region quota-inspector-test
// ask_model custa 30 unidades, e cada agente tem 100 por hora: a
// quarta pergunta seguida estoura o orçamento, e outro agente tem o
// próprio. web_search cai na taxa "*", de 60/m, e a 61.ª chamada
// seguida estoura a cota.
func TestInspect_cotaECusto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		i := New(Limits{
			Calls:  map[string]Rate{"*": rate(t, "60/m")},
			Cost:   map[string]int64{"ask_model": 30},
			Budget: new(rate(t, "100/h")),
		})
		for _, c := range []struct {
			agent, tool string
			n, want     int
		}{
			{"relatorios", "ask_model", 4, 3},
			{"suporte", "ask_model", 4, 3},
			{"monitor", "web_search", 61, 60},
		} {
			got, findings := count(t, i, c.agent, c.tool, c.n)
			t.Logf("%-10s %-10s %2d de %2d passaram; %v", c.agent,
				c.tool, got, c.n, findings)
			if got != c.want {
				t.Errorf("%s %s: %d", c.agent, c.tool, got)
			}
		}
	})
}

// #endregion
