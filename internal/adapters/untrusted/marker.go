// Package untrusted marca o conteúdo que veio de uma ferramenta como
// não confiável, para que o modelo saiba onde ele começa, onde termina
// e de onde veio.
package untrusted

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var _ app.ResultMarker = (*Marker)(nil)

// Marker envolve cada bloco de texto do resultado entre delimitadores.
type Marker struct {
	id func() string // um identificador novo por resultado
}

// New cria o marcador com identificadores de crypto/rand.
func New() *Marker { return &Marker{id: rand.Text} }

// #region untrusted-mark
// Mark envolve o texto de cada bloco do tipo text. O identificador é
// sorteado a cada resultado: o conteúdo não tem como saber qual será, e
// um delimitador de fim falso dentro dele não fecha nada.
func (m *Marker) Mark(
	input domain.PolicyInput, result json.RawMessage,
) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(result, &fields); err != nil {
		return nil, fmt.Errorf("untrusted: %w", err)
	}
	var content []map[string]any
	if err := json.Unmarshal(fields["content"], &content); err != nil {
		return result, nil // sem blocos de conteúdo, nada a marcar
	}
	id := m.id()
	for _, block := range content {
		text, ok := block["text"].(string)
		if ok && block["type"] == "text" {
			block["text"] = wrap(id, input, text)
		}
	}
	fields["content"], _ = json.Marshal(content)
	return json.Marshal(fields)
}

// #endregion

// #region untrusted-wrap
// wrap escreve os delimitadores. O de abertura diz de onde veio o
// texto; os dois levam o mesmo identificador.
func wrap(id string, input domain.PolicyInput, text string) string {
	return fmt.Sprintf("[[conteúdo não confiável id=%s servidor=%q "+
		"ferramenta=%q]]\n%s\n[[fim do conteúdo não confiável id=%s]]",
		id, input.Server, input.Call.ToolName, text, id)
}

// #endregion
