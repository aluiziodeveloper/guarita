// Package pipeline monta a linha de inspetores de uma rota a partir de
// uma lista de nomes, na ordem escrita pelo operador. O registro diz
// quem cria cada etapa; a montagem não conhece nenhuma delas.
package pipeline

import (
	"errors"
	"fmt"
	"slices"

	"github.com/aluiziodeveloper/guarita/internal/app"
)

// Erros da montagem. Todos aparecem na partida, e não na primeira
// chamada do agente.
var (
	ErrUnknownStage   = errors.New("pipeline: unknown stage")
	ErrDuplicateStage = errors.New("pipeline: stage listed twice")
	ErrMarkerNotLast  = errors.New("pipeline: marker must be last")
	ErrEmptyStage     = errors.New("pipeline: stage has no inspector")
)

// #region pipeline-stage
// Stage é uma etapa da linha: um inspetor da ida, um da volta ou o
// marcador do resultado. Cada fábrica preenche um dos três.
type Stage struct {
	Call   app.Inspector
	Result app.ResultInspector
	Marker app.ResultMarker
}

// Factory cria a etapa quando a rota é montada. O erro diz o que falta
// na configuração, como o catálogo para o esquema.
type Factory func() (Stage, error)

// Registry associa cada nome de etapa à fábrica dela. Uma etapa nova é
// uma entrada nova; Build não muda.
type Registry map[string]Factory

// #endregion

// #region pipeline-type
// Pipeline é a linha montada de uma rota: os nomes, na ordem dada, e o
// que cada sentido recebe.
type Pipeline struct {
	Names  []string
	Call   []app.Inspector
	Result []app.ResultInspector
	Marker app.ResultMarker
}

// #endregion

// #region pipeline-build
// Build cria cada etapa na ordem dos nomes. Um nome fora do registro,
// repetido ou um marcador antes do fim recusam a rota inteira.
func (r Registry) Build(names []string) (Pipeline, error) {
	p := Pipeline{Names: slices.Clone(names)}
	for n, name := range names {
		factory, ok := r[name]
		if !ok {
			return Pipeline{}, fmt.Errorf("%w: %q", ErrUnknownStage,
				name)
		}
		if slices.Contains(names[:n], name) {
			return Pipeline{}, fmt.Errorf("%w: %q", ErrDuplicateStage,
				name)
		}
		stage, err := factory()
		if err != nil {
			return Pipeline{}, fmt.Errorf("pipeline: %s: %w", name, err)
		}
		if err := p.add(stage, n == len(names)-1); err != nil {
			return Pipeline{}, fmt.Errorf("%w: %q", err, name)
		}
	}
	return p, nil
}

// #endregion

// add põe a etapa no sentido dela. O marcador fecha a volta: o que
// vem depois dele leria o texto já marcado.
func (p *Pipeline) add(stage Stage, last bool) error {
	switch {
	case stage.Marker != nil && !last:
		return ErrMarkerNotLast
	case stage.Marker != nil:
		p.Marker = stage.Marker
	case stage.Call != nil:
		p.Call = append(p.Call, stage.Call)
	case stage.Result != nil:
		p.Result = append(p.Result, stage.Result)
	default:
		return ErrEmptyStage
	}
	return nil
}

// #region pipeline-options
// Options devolve as opções do caso de uso, na ordem montada.
func (p Pipeline) Options() []app.Option {
	options := []app.Option{
		app.WithInspectors(p.Call...),
		app.WithResultInspectors(p.Result...),
	}
	if p.Marker != nil {
		options = append(options, app.WithResultMarker(p.Marker))
	}
	return options
}

// ReadsResults diz se a rota precisa abrir as respostas de tools/call.
func (p Pipeline) ReadsResults() bool {
	return len(p.Result) > 0 || p.Marker != nil
}

// #endregion

// #region pipeline-missing
// Missing devolve as etapas exigidas que a rota não tem. É a
// conferência da lacuna: uma rota sem a camada que todas deveriam ter
// é recusada na partida, em vez de ser descoberta por um vazamento.
func (p Pipeline) Missing(required []string) []string {
	var missing []string
	for _, name := range required {
		if !slices.Contains(p.Names, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

// #endregion
