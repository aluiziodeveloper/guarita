// Package injection examina o resultado de uma ferramenta em busca de
// texto escrito para o modelo, e não para quem pediu: marcas de papel
// que imitam a conversa, caracteres que não aparecem e marcação HTML
// que esconde texto.
package injection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var _ app.ResultInspector = (*Inspector)(nil)

// Sinais do inspetor.
const (
	SignalHidden       = "hidden"        // caractere invisível ou de controle
	SignalRoleMarker   = "role-marker"   // imita uma mensagem da conversa
	SignalHiddenMarkup = "hidden-markup" // HTML que esconde texto
)

// must compila na inicialização: um padrão errado derruba a partida.
var must = regexp.MustCompile

// #region injection-role-markers
// roleMarkers são as formas que um texto usa para se passar por uma
// mensagem do sistema, do assistente ou de quem escreve as instruções:
// um rótulo de papel no começo da linha, uma etiqueta de papel, os
// delimitadores dos modelos de conversa abertos.
var roleMarkers = []*regexp.Regexp{
	must(`(?im)^\s*(?:system|sistema|assistant|assistente|` +
		`developer)\s*:`),
	must(`(?i)<\s*/?\s*(?:system|assistant|developer|` +
		`instructions?)\b[^>]*>`),
	must(`\[/?(?:INST|SYS)\]|<</?SYS>>|<\|im_(?:start|end)\|>`),
	must(`(?im)^\s*#{2,}\s*(?:system|instructions?)\b`),
}

// #endregion

// #region injection-hidden-markup
// hiddenMarkup são as formas de pôr texto numa página sem mostrá-lo:
// um comentário com palavras e um estilo que some com o elemento.
var hiddenMarkup = []*regexp.Regexp{
	must(`<!--[\s\S]*?\pL[\s\S]*?-->`),
	must(`(?i)style\s*=\s*["'][^"']*(?:display\s*:\s*none|` +
		`visibility\s*:\s*hidden|font-size\s*:\s*0)`),
}

// #endregion

// patterns liga cada sinal às suas formas.
var patterns = []struct {
	signal string
	forms  []*regexp.Regexp
}{{SignalRoleMarker, roleMarkers}, {SignalHiddenMarkup, hiddenMarkup}}

// Actions é a decisão de cada sinal. Num resultado, qualquer coisa que
// não seja allow barra o resultado inteiro.
type Actions map[string]domain.Decision

// DefaultActions barra os três sinais.
func DefaultActions() Actions {
	return Actions{
		SignalHidden:       domain.DecisionDeny,
		SignalRoleMarker:   domain.DecisionDeny,
		SignalHiddenMarkup: domain.DecisionDeny,
	}
}

// Inspector é o inspetor de injeção nos resultados.
type Inspector struct{ actions Actions }

// New cria o inspetor com as decisões dadas.
func New(actions Actions) *Inspector { return &Inspector{actions: actions} }

// #region injection-inspect
// InspectResult percorre todas as strings do resultado: o texto de cada
// bloco de conteúdo, o conteúdo estruturado e os recursos embutidos.
func (i *Inspector) InspectResult(
	_ context.Context, _ domain.PolicyInput, result json.RawMessage,
) (domain.Inspection, error) {
	inspection := domain.Inspection{Decision: domain.DecisionAllow}
	decoder := json.NewDecoder(bytes.NewReader(result))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return inspection, fmt.Errorf("injection: %w", err)
	}
	walk("", value, func(field, text string) {
		for _, f := range detect(field, text) {
			inspection.Findings = append(inspection.Findings, f)
			inspection.Decision = domain.Stricter(inspection.Decision,
				i.action(f.Signal))
		}
	})
	return inspection, nil
}

// #endregion

func (i *Inspector) action(signal string) domain.Decision {
	if decision, ok := i.actions[signal]; ok {
		return decision
	}
	return domain.DecisionDeny
}

// #region injection-detect
// detect aplica os detectores a um texto. Cada um relata uma vez por
// campo, com a forma que achou, e nunca o texto injetado em si.
func detect(field, text string) []domain.Finding {
	var findings []domain.Finding
	if r, found := hiddenRune(text); found {
		findings = append(findings, domain.Finding{
			Field:  field,
			Signal: SignalHidden, Detail: fmt.Sprintf("U+%04X", r),
		})
	}
	for _, group := range patterns {
		for n, form := range group.forms {
			if form.MatchString(text) {
				findings = append(findings, domain.Finding{
					Field: field, Signal: group.signal,
					Detail: fmt.Sprintf("forma %d", n+1),
				})
				break
			}
		}
	}
	return findings
}

// #endregion

// hiddenRune acha o primeiro caractere que não aparece: formatação
// (Cf) ou controle (Cc) fora de quebra de linha e tabulação.
func hiddenRune(text string) (rune, bool) {
	for _, r := range text {
		if unicode.Is(unicode.Cf, r) || (unicode.Is(unicode.Cc, r) &&
			!strings.ContainsRune("\n\r\t", r)) {
			return r, true
		}
	}
	return 0, false
}

// walk chama visit para cada string do valor, com o caminho dela.
func walk(path string, value any, visit func(field, text string)) {
	switch v := value.(type) {
	case string:
		visit(path, v)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			walk(strings.TrimPrefix(path+"."+key, "."), v[key], visit)
		}
	case []any:
		for n, item := range v {
			walk(fmt.Sprintf("%s[%d]", path, n), item, visit)
		}
	}
}
