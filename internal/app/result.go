package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ErrResultBlocked indica que um inspetor barrou o resultado de uma
// chamada antes de ele chegar ao agente.
var ErrResultBlocked = errors.New("tool result blocked by inspection")

// #region app-result-ports
// ResultInspector examina o resultado de tools/call, o objeto result da
// resposta do servidor, antes de ele chegar ao agente.
type ResultInspector interface {
	InspectResult(
		ctx context.Context, input domain.PolicyInput,
		result json.RawMessage,
	) (domain.Inspection, error)
}

// ResultMarker marca o resultado que passou, para que o modelo saiba de
// onde veio cada texto.
type ResultMarker interface {
	Mark(
		input domain.PolicyInput, result json.RawMessage,
	) (json.RawMessage, error)
}

// #endregion

// WithResultInspectors acrescenta inspetores de resultado, em ordem.
func WithResultInspectors(inspectors ...ResultInspector) Option {
	return func(uc *InterceptCall) {
		uc.resultInspectors = append(uc.resultInspectors, inspectors...)
	}
}

// WithResultMarker marca todo resultado que passou pelos inspetores.
func WithResultMarker(marker ResultMarker) Option {
	return func(uc *InterceptCall) { uc.marker = marker }
}

// #region app-review-result
// ReviewResult decide o que do resultado chega ao agente. Um inspetor
// que não permite barra o resultado inteiro: não há aprovação humana
// no caminho de volta, porque o agente já está esperando. O que passa
// é marcado, se houver marcador.
func (uc *InterceptCall) ReviewResult(
	ctx context.Context, call domain.Call, result json.RawMessage,
) (json.RawMessage, error) {
	input := uc.input(ctx, call)
	_, stopper, err := runStages(uc.resultInspectors,
		func(i ResultInspector) (domain.Inspection, error) {
			return i.InspectResult(ctx, input, result)
		},
		func(in domain.Inspection) { uc.findings(input, in) },
		func(in domain.Inspection) bool {
			return in.Decision != domain.DecisionAllow
		})
	if err != nil {
		return nil, err
	}
	if stopper != nil {
		return nil, fmt.Errorf("%w: %s", ErrResultBlocked,
			summary(stopper.Findings))
	}
	if uc.marker == nil {
		return result, nil
	}
	return uc.marker.Mark(input, result)
}

// #endregion
