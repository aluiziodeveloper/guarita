package mcphttp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ToolReviewer é o catálogo visto daqui: recebe as definições de
// tools/list e devolve as que podem chegar ao agente.
type ToolReviewer interface {
	Review(ctx context.Context, definitions []domain.ToolDefinition) (
		[]domain.ToolDefinition, error)
}

// Erros da filtragem: a resposta do servidor não pôde ser lida, ou o
// catálogo não respondeu. Nos dois casos, nada chega ao agente.
var (
	errUpstreamResponse = errors.New("invalid upstream response")
	errCatalog          = errors.New("catalog unavailable")
)

// listToolsKey marca, no contexto do pedido de saída, que ele é um
// tools/list: só essas respostas são lidas e filtradas.
type listToolsKey struct{}

// WithCatalog faz a Guarita filtrar as respostas de tools/list.
func WithCatalog(reviewer ToolReviewer) Option {
	return func(h *Handler) { h.catalog = reviewer }
}

// rewrite reescreve uma mensagem JSON-RPC do servidor. Cada tipo de
// resposta que a Guarita lê tem a sua: a de tools/list filtra pelo
// catálogo; as outras ficam para os capítulos seguintes.
type rewrite func(ctx context.Context, data []byte) ([]byte, error)

// #region mcphttp-modify-response
// modifyResponse escolhe a reescrita pelo pedido e a aplica à resposta,
// em JSON ou em fluxo. Sem reescrita, a resposta passa sem ser lida.
func (h *Handler) modifyResponse(response *http.Response) error {
	ctx := response.Request.Context()
	change := h.rewriteFor(ctx)
	if change == nil || response.StatusCode != http.StatusOK {
		return nil
	}
	defer func() { _ = response.Body.Close() }()
	body := io.LimitReader(response.Body, maxBodyBytes)
	var changed []byte
	var err error
	if strings.HasPrefix(response.Header.Get("Content-Type"),
		"text/event-stream") {
		changed, err = rewriteStream(ctx, bufio.NewReader(body), change)
	} else {
		changed, err = rewriteBody(ctx, body, change)
	}
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(changed))
	response.ContentLength = int64(len(changed))
	response.Header.Set("Content-Length", strconv.Itoa(len(changed)))
	return nil
}

// #endregion

// rewriteFor devolve a reescrita do pedido, ou nil.
func (h *Handler) rewriteFor(ctx context.Context) rewrite {
	if h.catalog != nil && ctx.Value(listToolsKey{}) != nil {
		return func(ctx context.Context, data []byte) ([]byte, error) {
			return filterMessage(ctx, h.catalog, data)
		}
	}
	return nil
}

func rewriteBody(
	ctx context.Context, body io.Reader, change rewrite,
) ([]byte, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return change(ctx, data)
}

// #region mcphttp-filter-stream
// rewriteStream lê o fluxo evento a evento até a resposta, reescreve as
// linhas data no caminho e para: não espera o servidor fechar.
func rewriteStream(
	ctx context.Context, reader *bufio.Reader, change rewrite,
) ([]byte, error) {
	var out bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			data := []byte(strings.TrimSpace(payload))
			filtered, ferr := change(ctx, data)
			if ferr != nil {
				return nil, ferr
			}
			line = "data: " + string(filtered) + "\n"
			if isResponse(filtered) {
				out.WriteString(line + "\n") // fecha o evento e encerra
				return out.Bytes(), nil
			}
		}
		out.WriteString(line)
		if err != nil {
			return nil, fmt.Errorf("%w: stream ended without response",
				errUpstreamResponse)
		}
	}
}

// #endregion

func isResponse(data []byte) bool {
	message, err := mcpproxy.ParseMessage(data)
	return err == nil && message.Kind == mcpproxy.KindResponse
}

// #region mcphttp-filter-message
// filterMessage troca result.tools pelas definições que o catálogo
// aprovou. Mensagens que não são resposta com tools passam iguais.
func filterMessage(
	ctx context.Context, reviewer ToolReviewer, data []byte,
) ([]byte, error) {
	message, err := mcpproxy.ParseMessage(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUpstreamResponse, err)
	}
	var result map[string]json.RawMessage
	if message.Kind != mcpproxy.KindResponse ||
		json.Unmarshal(message.Result, &result) != nil ||
		result["tools"] == nil {
		return data, nil
	}
	definitions, err := toolDefinitions(result["tools"])
	if err != nil {
		return nil, err
	}
	kept, err := reviewer.Review(ctx, definitions)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCatalog, err)
	}
	result["tools"] = joinTools(kept)
	message.Result, _ = json.Marshal(result)
	return json.Marshal(message)
}

// #endregion

// toolDefinitions separa a lista em definições, com o nome de cada uma.
func toolDefinitions(raw json.RawMessage) ([]domain.ToolDefinition, error) {
	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("%w: tools: %w", errUpstreamResponse, err)
	}
	definitions := make([]domain.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		var head struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(tool, &head) // sem nome, o catálogo recusa
		definitions = append(definitions,
			domain.ToolDefinition{Name: head.Name, Raw: tool})
	}
	return definitions, nil
}

func joinTools(kept []domain.ToolDefinition) json.RawMessage {
	raws := make([]json.RawMessage, 0, len(kept))
	for _, definition := range kept {
		raws = append(raws, definition.Raw)
	}
	joined, _ := json.Marshal(raws)
	return joined
}
