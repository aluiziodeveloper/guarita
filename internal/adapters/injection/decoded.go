package injection

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var _ app.ResultInspector = (*Decoded)(nil)

// base64Run acha um trecho que pode ser base64: 24 caracteres ou
// mais do alfabeto, com até dois de preenchimento.
var base64Run = regexp.MustCompile(`[A-Za-z0-9+/]{24,}={0,2}`)

// #region injection-decoded
// Decoded lê o que está em base64 no texto do resultado. Cada trecho
// que decodifica para texto passa pelos detectores do inspetor dado,
// com as decisões dele; o achado diz que veio de dentro do base64.
type Decoded struct{ inner *Inspector }

// NewDecoded cria a etapa sobre as regras e decisões de inner.
func NewDecoded(inner *Inspector) *Decoded { return &Decoded{inner} }

// #endregion

// #region injection-decoded-inspect
// InspectResult decodifica e inspeciona cada trecho de cada bloco.
func (d *Decoded) InspectResult(
	_ context.Context, _ domain.PolicyInput, result json.RawMessage,
) (domain.Inspection, error) {
	inspection := domain.Inspection{Decision: domain.DecisionAllow}
	var r struct {
		Content []struct{ Type, Text string } `json:"content"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return inspection, fmt.Errorf("decoded: %w", err)
	}
	for n, c := range r.Content {
		field := fmt.Sprintf("content[%d].text (base64)", n)
		for _, run := range base64Run.FindAllString(c.Text, -1) {
			plain, ok := decodeText(run)
			if !ok {
				continue
			}
			for _, f := range detect(field, plain) {
				inspection.Findings = append(inspection.Findings, f)
				inspection.Decision = domain.Stricter(
					inspection.Decision, d.inner.action(f.Signal))
			}
		}
	}
	return inspection, nil
}

// #endregion

// #region injection-decode-text
// decodeText aceita o trecho que decodifica para UTF-8 válido, sem
// caracteres de controle além de quebras e tabulação. Os caracteres de
// formatação ficam: são o sinal hidden. Um resumo SHA-256 em base64
// vira bytes soltos e não passa.
func decodeText(run string) (string, bool) {
	raw, err := base64.StdEncoding.DecodeString(run)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(
			strings.TrimRight(run, "="))
	}
	if err != nil || !utf8.Valid(raw) {
		return "", false
	}
	for _, r := range string(raw) {
		if unicode.IsControl(r) && !strings.ContainsRune("\n\r\t", r) {
			return "", false
		}
	}
	return string(raw), true
}

// #endregion
