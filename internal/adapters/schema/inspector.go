// Package schema confere os argumentos de uma chamada contra o
// inputSchema da definição fixada no catálogo: o contrato que o
// servidor anunciou e que o operador aceitou.
package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// SignalSchema é um argumento fora do inputSchema fixado.
const SignalSchema = "schema"

// ErrNoSchema indica uma definição fixada sem inputSchema.
var ErrNoSchema = errors.New("pinned definition has no inputSchema")

var _ app.Inspector = (*Inspector)(nil)

// Definitions é de onde vem a definição fixada; *app.PinCatalog a dá.
type Definitions interface {
	Pinned(ctx context.Context, tool string) (json.RawMessage, error)
}

// Inspector valida os argumentos e guarda os esquemas já resolvidos.
type Inspector struct {
	definitions Definitions
	mu          sync.Mutex
	resolved    map[string]*jsonschema.Resolved // pelo texto do esquema
}

// New cria o inspetor sobre o catálogo de um servidor.
func New(definitions Definitions) *Inspector {
	return &Inspector{
		definitions: definitions, resolved: map[string]*jsonschema.Resolved{},
	}
}

// #region schema-inspect
// Inspect valida os argumentos contra o inputSchema fixado. O esquema
// que não se resolve, como o que aponta um $ref remoto, é erro: sem
// Loader, a biblioteca não busca nada na rede.
func (i *Inspector) Inspect(
	ctx context.Context, input domain.PolicyInput,
) (domain.Inspection, error) {
	allow := domain.Inspection{Decision: domain.DecisionAllow}
	resolved, err := i.schemaFor(ctx, input.Call.ToolName)
	if err != nil {
		return allow, fmt.Errorf("schema: %w", err)
	}
	var arguments any
	decoder := json.NewDecoder(bytes.NewReader(input.Call.Arguments))
	decoder.UseNumber()
	if err := decoder.Decode(&arguments); err != nil {
		return allow, fmt.Errorf("schema: %w", err)
	}
	if err := resolved.Validate(arguments); err != nil {
		return domain.Inspection{
			Decision: domain.DecisionDeny,
			Findings: []domain.Finding{{
				Field:  "arguments",
				Signal: SignalSchema, Detail: withoutValues(err),
			}},
		}, nil
	}
	return allow, nil
}

// #endregion

// schemaFor lê o inputSchema da definição fixada e o resolve uma vez.
func (i *Inspector) schemaFor(
	ctx context.Context, tool string,
) (*jsonschema.Resolved, error) {
	raw, err := i.definitions.Pinned(ctx, tool)
	if err != nil {
		return nil, err
	}
	var definition struct {
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil ||
		len(definition.InputSchema) == 0 {
		return nil, ErrNoSchema
	}
	key := string(definition.InputSchema)
	i.mu.Lock()
	defer i.mu.Unlock()
	if resolved, ok := i.resolved[key]; ok {
		return resolved, nil
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(definition.InputSchema, &s); err != nil {
		return nil, err
	}
	resolved, err := s.Resolve(nil)
	if err != nil {
		return nil, err
	}
	i.resolved[key] = resolved
	return resolved, nil
}

// location acha o último "validating <lugar>: " da mensagem.
var location = regexp.MustCompile(`validating (\S+): `)

// #region schema-without-values
// withoutValues reduz o erro ao lugar no esquema e à palavra-chave que
// falhou. A mensagem da biblioteca traz os valores dos argumentos, e o
// relatório não pode carregá-los.
func withoutValues(err error) string {
	message := err.Error()
	place := "root"
	places := location.FindAllStringSubmatchIndex(message, -1)
	if places != nil {
		last := places[len(places)-1] // o lugar mais fundo no esquema
		place, message = message[last[2]:last[3]], message[last[1]:]
	}
	keyword, _, found := strings.Cut(message, ":")
	if !found || strings.ContainsAny(keyword, ` "[`) {
		keyword = strings.Join(strings.Fields(message)[:min(3,
			len(strings.Fields(message)))], " ")
		keyword, _, _ = strings.Cut(keyword, "[")
	}
	return place + " " + strings.TrimSpace(keyword)
}

// #endregion
