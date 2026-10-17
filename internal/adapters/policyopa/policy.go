// Package policyopa decide as chamadas com políticas em Rego, avaliadas
// pelo OPA embutido como biblioteca: a decisão acontece no processo da
// Guarita, sem rede e sem um serviço a mais para operar.
package policyopa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Query é a regra que a Guarita pergunta à política.
const Query = "data.guarita.decision"

var (
	// ErrNoModules recusa um diretório de política sem arquivo .rego.
	ErrNoModules = errors.New("policy has no .rego module")
	// ErrUndefined indica que a política não produziu decisão: a regra
	// não existe ou nenhum corpo casou e não há valor padrão.
	ErrUndefined = errors.New("policy decision is undefined")
)

var _ app.PolicyEvaluator = (*Policy)(nil)

// Policy é a consulta já preparada: compilada uma vez, na partida.
type Policy struct {
	query rego.PreparedEvalQuery
}

// #region policyopa-load
// Load lê os módulos .rego de fsys, menos os testes, e prepara a
// consulta. Um erro de sintaxe ou de tipo aparece aqui, na partida, e
// não na primeira chamada do agente.
func Load(ctx context.Context, fsys fs.FS) (*Policy, error) {
	options := []func(*rego.Rego){
		rego.Query(Query), rego.StrictBuiltinErrors(true),
	}
	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry,
		err error,
	) error {
		if err != nil || entry.IsDir() || !isModule(name) {
			return err
		}
		source, err := fs.ReadFile(fsys, name)
		options = append(options, rego.Module(name, string(source)))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load policy: %w", err)
	}
	if len(options) == 2 {
		return nil, ErrNoModules
	}
	query, err := rego.New(options...).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile policy: %w", err)
	}
	return &Policy{query: query}, nil
}

// #endregion

// isModule aceita as regras e deixa de fora os testes de opa test.
func isModule(name string) bool {
	return path.Ext(name) == ".rego" &&
		!strings.HasSuffix(name, "_test.rego")
}

// #region policyopa-evaluate
// Evaluate entrega a entrada à consulta preparada e lê a decisão. Sem
// resultado, ou com um que não é texto, a política falhou, e o caso de
// uso nega.
func (p *Policy) Evaluate(
	ctx context.Context, input domain.PolicyInput,
) (domain.Decision, error) {
	document, err := toDocument(input)
	if err != nil {
		return "", err
	}
	results, err := p.query.Eval(ctx, rego.EvalInput(document))
	if err != nil {
		return "", fmt.Errorf("evaluate policy: %w", err)
	}
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return "", ErrUndefined
	}
	decision, ok := results[0].Expressions[0].Value.(string)
	if !ok {
		return "", fmt.Errorf("%w: decision is %T",
			ErrUndefined, results[0].Expressions[0].Value)
	}
	return domain.Decision(decision), nil
}

// #endregion

// #region policyopa-document
// toDocument escreve a entrada como o documento input da política. Os
// argumentos vão decodificados, para que as regras leiam os campos, e
// com UseNumber, para que um número grande não perca precisão.
func toDocument(input domain.PolicyInput) (map[string]any, error) {
	var arguments any = map[string]any{}
	if raw := input.Call.Arguments; len(raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&arguments); err != nil {
			return nil, fmt.Errorf("policy input: %w", err)
		}
	}
	return map[string]any{
		"agent":     input.Agent,
		"server":    input.Server,
		"tool":      input.Call.ToolName,
		"arguments": arguments,
	}, nil
}

// #endregion
