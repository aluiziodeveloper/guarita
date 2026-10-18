// Package dlp examina os argumentos de uma chamada em busca de dados
// que não deveriam sair: CPF, chaves de API, chaves privadas, segredos
// codificados e, desde a seção de destinos, URLs fora da lista.
package dlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var _ app.Inspector = (*Inspector)(nil)

// Sinais do inspetor.
const (
	SignalCPF     = "cpf"
	SignalSecret  = "secret"
	SignalEntropy = "entropy"
)

// #region dlp-actions
// Actions é a decisão de cada sinal. As de DefaultActions refletem o
// custo de errar: uma chave tem formato inequívoco e nega; CPF e
// entropia alta têm usos legítimos e pedem aprovação.
type Actions map[string]domain.Decision

// DefaultActions são as decisões do laboratório.
func DefaultActions() Actions {
	return Actions{
		SignalSecret:  domain.DecisionDeny,
		SignalCPF:     domain.DecisionRequireApproval,
		SignalEntropy: domain.DecisionRequireApproval,
	}
}

// #endregion

// Inspector é o inspetor DLP.
type Inspector struct {
	actions    Actions
	minEntropy float64
}

// New cria o inspetor. minEntropy é o limiar, em bits por caractere,
// para um token de 24 caracteres ou mais ser tratado como segredo.
func New(actions Actions, minEntropy float64) *Inspector {
	return &Inspector{actions: actions, minEntropy: minEntropy}
}

// #region dlp-inspect
// Inspect percorre todas as strings dos argumentos, chaves e valores,
// e junta os achados. A decisão é a mais severa entre as dos sinais.
func (i *Inspector) Inspect(
	_ context.Context, input domain.PolicyInput,
) (domain.Inspection, error) {
	inspection := domain.Inspection{Decision: domain.DecisionAllow}
	decoder := json.NewDecoder(bytes.NewReader(input.Call.Arguments))
	decoder.UseNumber()
	var arguments any
	if err := decoder.Decode(&arguments); err != nil {
		return inspection, fmt.Errorf("dlp: %w", err)
	}
	walk("", arguments, func(field, text string) {
		for _, f := range i.detect(field, text) {
			inspection.Findings = append(inspection.Findings, f)
			inspection.Decision = domain.Stricter(inspection.Decision,
				i.action(f.Signal))
		}
	})
	return inspection, nil
}

// #endregion

// action devolve a decisão do sinal; um sinal sem decisão nega.
func (i *Inspector) action(signal string) domain.Decision {
	if decision, ok := i.actions[signal]; ok {
		return decision
	}
	return domain.DecisionDeny
}

// #region dlp-detect
// detect aplica os detectores a um texto e, uma vez, ao que estiver
// codificado em base64 dentro dele. O detalhe diz o que foi achado,
// nunca o valor.
func (i *Inspector) detect(field, text string) []domain.Finding {
	findings := direct(field, text)
	for _, candidate := range token.FindAllString(text, -1) {
		if plain, ok := decoded(candidate); ok {
			inner := direct(field+" (base64)", plain)
			findings = append(findings, inner...)
		}
		bits := entropy(candidate)
		if len(candidate) >= 24 && bits >= i.minEntropy {
			findings = append(findings, domain.Finding{
				Field: field, Signal: SignalEntropy,
				Detail: fmt.Sprintf("%.1f bits/caractere em %d", bits,
					len(candidate)),
			})
		}
	}
	return findings
}

// #endregion

// direct aplica os detectores de CPF e de formatos de chave.
func direct(field, text string) []domain.Finding {
	var findings []domain.Finding
	for _, candidate := range cpfCandidate.FindAllString(text, -1) {
		if ValidCPF(candidate) {
			findings = append(findings, domain.Finding{
				Field:  field,
				Signal: SignalCPF, Detail: "dígitos verificadores válidos",
			})
		}
	}
	for _, p := range secretPatterns {
		if p.pattern.MatchString(text) {
			findings = append(findings, domain.Finding{
				Field:  field,
				Signal: SignalSecret, Detail: p.name,
			})
		}
	}
	return findings
}

// walk chama visit para cada string do valor, com o caminho dela,
// visitando as chaves de objeto em ordem e também como texto.
func walk(path string, value any, visit func(field, text string)) {
	switch v := value.(type) {
	case string:
		visit(path, v)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			field := strings.TrimPrefix(path+"."+key, ".")
			visit(field+" (chave)", key)
			walk(field, v[key], visit)
		}
	case []any:
		for n, item := range v {
			walk(fmt.Sprintf("%s[%d]", path, n), item, visit)
		}
	}
}
