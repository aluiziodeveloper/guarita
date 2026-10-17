package mcpproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// maxDepth limita o aninhamento de objetos e arrays numa mensagem.
const maxDepth = 64

// Erros do parser. Todos recusam a mensagem inteira.
var (
	ErrNotUTF8      = errors.New("message is not valid UTF-8")
	ErrBatch        = errors.New("JSON-RPC batches are not supported")
	ErrMalformed    = errors.New("message is not valid JSON")
	ErrTrailingData = errors.New("data after the JSON-RPC message")
	ErrDuplicateKey = errors.New("duplicate key in JSON object")
	ErrTooDeep      = errors.New("message nesting is too deep")
	ErrVersion      = errors.New(`jsonrpc must be "2.0"`)
	ErrShape        = errors.New("not a request, notification or response")
	// ErrToolCallNotification recusa tools/call sem id, que fugiria da
	// política e não teria como receber a resposta de negação.
	ErrToolCallNotification = errors.New("tools/call must be a request")
)

// Kind classifica uma mensagem JSON-RPC.
type Kind int

const (
	// KindInvalid é o valor zero e nunca descreve uma mensagem aceita.
	KindInvalid Kind = iota
	KindRequest
	KindNotification
	KindResponse
)

// String devolve o nome usado nos testes e nos registros.
func (kind Kind) String() string {
	switch kind {
	case KindInvalid:
		return "invalid"
	case KindRequest:
		return "request"
	case KindNotification:
		return "notification"
	case KindResponse:
		return "response"
	default:
		return "Kind(" + strconv.Itoa(int(kind)) + ")"
	}
}

// #region mcpproxy-message
// Message é o envelope JSON-RPC 2.0 das mensagens MCP. Params, Result e
// Error seguem codificados: o parser confere a forma, não o conteúdo.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	Kind    Kind            `json:"-"`
}

// #endregion

// #region mcpproxy-parse
// ParseMessage aceita exatamente uma mensagem JSON-RPC 2.0, sem nada
// depois dela, sem chave repetida e em UTF-8 válido.
func ParseMessage(data []byte) (Message, error) {
	if !utf8.Valid(data) {
		return Message{}, ErrNotUTF8
	}
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '[' {
		return Message{}, ErrBatch
	}
	if err := checkSingleValue(data); err != nil {
		return Message{}, err
	}
	if err := checkKeys(newDecoder(data), 0); err != nil {
		return Message{}, err
	}
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if message.JSONRPC != "2.0" {
		return Message{}, ErrVersion
	}
	message.Kind = classify(message)
	if message.Kind == KindInvalid {
		return Message{}, ErrShape
	}
	return message, nil
}

// #endregion

// #region mcpproxy-single-value
// checkSingleValue recusa o contrabando: depois do primeiro valor JSON,
// só pode haver o fim da entrada.
func checkSingleValue(data []byte) error {
	decoder := newDecoder(data)
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrTrailingData
	}
	return nil
}

// #endregion

// #region mcpproxy-check-keys
// checkKeys percorre o valor token a token e recusa chave repetida no
// mesmo objeto e aninhamento além de maxDepth.
func checkKeys(decoder *json.Decoder, depth int) error {
	if depth > maxDepth {
		return ErrTooDeep
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	delim, ok := token.(json.Delim)
	if !ok || (delim != '{' && delim != '[') {
		return nil
	}
	seen := make(map[string]bool)
	for decoder.More() {
		if delim == '{' {
			if err := checkKey(decoder, seen); err != nil {
				return err
			}
		}
		if err := checkKeys(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token() // fecha o objeto ou o array
	return err
}

// #endregion

// checkKey lê a próxima chave de um objeto e recusa a repetida.
func checkKey(decoder *json.Decoder, seen map[string]bool) error {
	key, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	name, _ := key.(string) // chave de objeto é sempre string
	if seen[name] {
		return fmt.Errorf("%w: %q", ErrDuplicateKey, name)
	}
	seen[name] = true
	return nil
}

func newDecoder(data []byte) *json.Decoder {
	return json.NewDecoder(bytes.NewReader(data))
}

// classify aplica as regras do JSON-RPC 2.0 aos campos presentes.
func classify(message Message) Kind {
	hasID := len(message.ID) > 0 && string(message.ID) != "null"
	switch {
	case message.Method != "" && hasID:
		return KindRequest
	case message.Method != "":
		return KindNotification
	case hasID && (len(message.Result) > 0) != (len(message.Error) > 0):
		return KindResponse
	default:
		return KindInvalid
	}
}

// #region mcpproxy-tool-call
// toolCall traduz um pedido tools/call para o domínio. É o único ponto
// em que o adaptador conhece a forma dos parâmetros do MCP.
func toolCall(message Message) (domain.Call, bool, error) {
	if message.Method != "tools/call" {
		return domain.Call{}, false, nil
	}
	if message.Kind != KindRequest { // sem id, não teria resposta
		return domain.Call{}, true, ErrToolCallNotification
	}
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		err = fmt.Errorf("tools/call params: %w", err)
		return domain.Call{}, true, err
	}
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage(`{}`) // são opcionais
	}
	call := domain.Call{ToolName: params.Name}
	call.Arguments = params.Arguments
	return call, true, nil
}

// #endregion

// metaVersion é a chave de _meta com a revisão do protocolo do pedido.
const metaVersion = "io.modelcontextprotocol/protocolVersion"

// #region mcpproxy-mirrored
// Mirrored são os campos do corpo que a revisão 2026-07-28 espelha em
// cabeçalhos HTTP: a versão em _meta e o nome em params.name ou uri.
type Mirrored struct {
	Version string
	Name    string
	HasName bool // tools/call, prompts/get e resources/read têm nome
}

// Mirrored lê os campos espelhados. O corpo continua a fonte de
// verdade; quem compara com os cabeçalhos é o adaptador HTTP.
func (message Message) Mirrored() Mirrored {
	var params struct {
		Name string                     `json:"name"`
		URI  string                     `json:"uri"`
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	_ = json.Unmarshal(message.Params, &params) // forma já conferida
	mirrored := Mirrored{}
	_ = json.Unmarshal(params.Meta[metaVersion], &mirrored.Version)
	switch message.Method {
	case "tools/call", "prompts/get":
		mirrored.Name, mirrored.HasName = params.Name, true
	case "resources/read":
		mirrored.Name, mirrored.HasName = params.URI, true
	}
	return mirrored
}

// #endregion
