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
	if message.Kind == mcpproxy.KindNotification {
		return reviewNotification(ctx, reviewer, call, message, data)
	}
	if message.Kind != mcpproxy.KindResponse {
		return data, nil
	}
	if len(message.Error) > 0 {
		return reviewError(ctx, reviewer, call, message)
	}
	reviewed, err := reviewer.ReviewResult(ctx, call, message.Result)
	if err != nil {
		reviewed, _ = json.Marshal(blockedResult(err))
	}
	message.Result = reviewed
	return json.Marshal(message)
}

// #endregion

// #region mcphttp-review-error
// reviewError passa o erro JSON-RPC pelos mesmos inspetores do
// resultado: a mensagem e os dados de um erro de protocolo também podem
// chegar ao modelo. Um erro barrado mantém o código e troca a mensagem
// pelo aviso da Guarita, sem os dados; os outros seguem como vieram,
// porque a marcação só sabe embrulhar blocos de conteúdo.
func reviewError(
	ctx context.Context, reviewer ResultReviewer, call domain.Call,
	message mcpproxy.Message,
) ([]byte, error) {
	_, err := reviewer.ReviewResult(ctx, call, message.Error)
	if err == nil {
		return json.Marshal(message)
	}
	var wire struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(message.Error, &wire)
	message.Error, _ = json.Marshal(map[string]any{
		"code":    wire.Code,
		"message": "[Guarita] erro bloqueado: " + err.Error(),
	})
	return json.Marshal(message)
}

// #endregion

// #region mcphttp-review-notification
// reviewNotification passa os parâmetros de uma notificação do fluxo
// da resposta pelos inspetores da volta. A barrada some do fluxo: a
// saída vazia diz a rewriteStream que não escreva o evento. A limpa
// segue como veio, sem marcação, porque não tem blocos de conteúdo.
func reviewNotification(
	ctx context.Context, reviewer ResultReviewer, call domain.Call,
	message mcpproxy.Message, data []byte,
) ([]byte, error) {
	if len(message.Params) == 0 {
		return data, nil
	}
	_, err := reviewer.ReviewResult(ctx, call, message.Params)
	if err != nil {
		return nil, nil //nolint:nilerr // barrar não é falhar o fluxo
	}
	return data, nil
}

// #endregion

// blockedResult é o resultado de ferramenta que substitui o barrado.
func blockedResult(err error) map[string]any {
	return map[string]any{"isError": true, "content": []map[string]string{
		{"type": "text", "text": "[Guarita] resultado bloqueado: " +
			err.Error()},
	}}
}
