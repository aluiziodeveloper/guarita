package mcphttp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ResultReviewer é o caso de uso visto daqui: recebe o resultado de uma
// chamada e devolve o que pode chegar ao agente. *app.InterceptCall o
// satisfaz.
type ResultReviewer interface {
	ReviewResult(ctx context.Context, call domain.Call,
		result json.RawMessage) (json.RawMessage, error)
}

// toolCallKey guarda, no contexto do pedido de saída, a chamada cujo
// resultado precisa ser revisado.
type toolCallKey struct{}

// WithResults faz a Guarita revisar o resultado de cada tools/call.
func WithResults(reviewer ResultReviewer) Option {
	return func(h *Handler) { h.results = reviewer }
}

// markRequest anota no contexto o que a resposta vai precisar: a marca
// de tools/list, para o catálogo, e a chamada, para a revisão.
func (h *Handler) markRequest(
	ctx context.Context, message mcpproxy.Message,
) context.Context {
	if message.Method == "tools/list" {
		return context.WithValue(ctx, listToolsKey{}, true)
	}
	if call, ok, err := mcpproxy.ToolCall(message); ok && err == nil &&
		h.results != nil {
		return context.WithValue(ctx, toolCallKey{}, call)
	}
	return ctx
}

// #region mcphttp-review-message
// reviewMessage troca o result de uma resposta pelo que o caso de uso
// devolveu. Um resultado barrado vira um erro de ferramenta, com
// isError: o modelo vê que a chamada falhou e por quê, e a sessão
// continua.
func reviewMessage(
	ctx context.Context, reviewer ResultReviewer, call domain.Call,
	data []byte,
) ([]byte, error) {
	message, err := mcpproxy.ParseMessage(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUpstreamResponse, err)
	}
	if message.Kind != mcpproxy.KindResponse ||
		len(message.Result) == 0 {
		return data, nil // notificações e erros passam
	}
	reviewed, err := reviewer.ReviewResult(ctx, call, message.Result)
	if err != nil {
		reviewed, _ = json.Marshal(blockedResult(err))
	}
	message.Result = reviewed
	return json.Marshal(message)
}

// #endregion

// blockedResult é o resultado de ferramenta que substitui o barrado.
func blockedResult(err error) map[string]any {
	return map[string]any{"isError": true, "content": []map[string]string{
		{"type": "text", "text": "[Guarita] resultado bloqueado: " +
			err.Error()},
	}}
}
