package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var _ app.Inspector = (*Inspector)(nil)

// Sinais do inspetor.
const (
	SignalQuota  = "quota"  // chamadas demais à ferramenta
	SignalBudget = "budget" // custo demais para o agente
)

// #region quota-limits
// Limits são as cotas: a taxa de chamadas de cada ferramenta, por
// agente e servidor ("*" vale para as ferramentas sem taxa própria), o
// custo estimado de cada chamada, em unidades que o operador define, e
// o orçamento de unidades de cada agente.
type Limits struct {
	Calls  map[string]Rate
	Cost   map[string]int64 // sem entrada, a chamada custa 1
	Budget *Rate            // nil: sem orçamento
}

// #endregion

// key identifica um balde. Uma estrutura de strings é comparável e
// serve de chave: "a:b" como agente não se confunde com outra divisão
// dos mesmos caracteres, como aconteceria numa chave concatenada.
type key struct{ kind, agent, server, tool string }

// Inspector aplica as cotas antes do repasse.
type Inspector struct {
	limits  Limits
	buckets sync.Map // key → *Bucket
}

// New cria o inspetor com os limites dados.
func New(limits Limits) *Inspector { return &Inspector{limits: limits} }

// #region quota-inspect
// Inspect tira uma ficha do balde da ferramenta e o custo da chamada
// do orçamento do agente. A primeira recusa nega a chamada, e o achado
// diz quanto esperar, sem os argumentos.
func (i *Inspector) Inspect(
	_ context.Context, input domain.PolicyInput,
) (domain.Inspection, error) {
	tool := input.Call.ToolName
	if r, ok := i.rate(tool); ok {
		k := key{"calls", input.Agent, input.Server, tool}
		if ok, wait := i.bucket(k, r).Take(1); !ok {
			return deny(SignalQuota, tool, wait), nil
		}
	}
	if b := i.limits.Budget; b != nil {
		k := key{kind: "budget", agent: input.Agent}
		if ok, wait := i.bucket(k, *b).Take(i.cost(tool)); !ok {
			return deny(SignalBudget, tool, wait), nil
		}
	}
	return domain.Inspection{Decision: domain.DecisionAllow}, nil
}

// #endregion

// #region quota-bucket-for
// bucket devolve o balde da chave, criado na primeira vez. sync.Map
// serve ao caso em que cada chave é escrita uma vez e lida muitas, sem
// uma trava para o mapa inteiro. LoadOrStore resolve a corrida de duas
// primeiras chamadas: as duas recebem o mesmo balde.
func (i *Inspector) bucket(k key, r Rate) *Bucket {
	if b, ok := i.buckets.Load(k); ok {
		return b.(*Bucket)
	}
	b, _ := i.buckets.LoadOrStore(k, NewBucket(r))
	return b.(*Bucket)
}

// #endregion

func (i *Inspector) rate(tool string) (Rate, bool) {
	if r, ok := i.limits.Calls[tool]; ok {
		return r, true
	}
	r, ok := i.limits.Calls["*"]
	return r, ok
}

func (i *Inspector) cost(tool string) int64 {
	if c, ok := i.limits.Cost[tool]; ok {
		return c
	}
	return 1
}

func deny(signal, tool string, wait time.Duration) domain.Inspection {
	return domain.Inspection{
		Decision: domain.DecisionDeny,
		Findings: []domain.Finding{{
			Field: tool, Signal: signal,
			Detail: fmt.Sprintf("esperar %v", wait.Round(time.Millisecond)),
		}},
	}
}
