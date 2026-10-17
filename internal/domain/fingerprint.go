package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidDefinition recusa uma definição de ferramenta que não é um
// único objeto JSON com nome.
var ErrInvalidDefinition = errors.New(
	"tool definition must be one JSON object with a name")

// ToolDefinition é a definição de uma ferramenta como o servidor a
// anuncia em tools/list: o nome e o objeto JSON inteiro, como veio.
type ToolDefinition struct {
	Name string
	Raw  json.RawMessage
}

// Fingerprint identifica uma definição: "sha256:" e o resumo em hex.
type Fingerprint string

// #region domain-fingerprint
// Fingerprint resume a definição inteira, e não só a descrição: nome,
// esquemas, anotações e qualquer campo que o servidor acrescente chegam
// ao modelo e precisam ficar fixados juntos.
func (d ToolDefinition) Fingerprint() (Fingerprint, error) {
	if d.Name == "" {
		return "", ErrInvalidDefinition
	}
	canonical, err := Canonical(d.Raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return Fingerprint("sha256:" + hex.EncodeToString(sum[:])), nil
}

// #endregion

// #region domain-canonical
// Canonical reescreve um objeto JSON numa forma única: chaves em ordem,
// sem espaços, números exatamente como vieram. Duas definições com o
// mesmo conteúdo têm os mesmos bytes, e por isso o mesmo resumo.
func Canonical(raw json.RawMessage) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber() // 1.0 e 1 não viram o mesmo float64
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDefinition, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidDefinition // há algo depois do objeto
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, ErrInvalidDefinition
	}
	return json.Marshal(value) // json.Marshal ordena as chaves de mapas
}

// #endregion
