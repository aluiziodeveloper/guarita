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

// Sinais do inspetor. Os três primeiros são fortes: um resultado
// legítimo quase nunca os tem. Os dois últimos são fracos: aparecem em
// transcrições, fichas técnicas e páginas comuns.
const (
	SignalHidden        = "hidden"         // caractere que não aparece
	SignalChatDelimiter = "chat-delimiter" // delimitador de conversa
	SignalHiddenStyle   = "hidden-style"   // estilo que some com o texto
	SignalRoleLabel     = "role-label"     // rótulo ou etiqueta de papel
	SignalHTMLComment   = "html-comment"   // comentário com palavras
)

// must compila na inicialização: um padrão errado derruba a partida.
var must = regexp.MustCompile

// #region injection-patterns
// patterns são as formas de cada sinal. Os delimitadores são os dos
// modelos de conversa abertos; os estilos, os que escondem um elemento;
// os rótulos, os que imitam quem fala numa conversa.
var patterns = []struct {
	signal string
	forms  []form
}{
	{SignalChatDelimiter, []form{{must(`\[/?(?:INST|SYS)\]|` +
		`<</?SYS>>|<\|im_(?:start|end)\|>`), []string{"[", "<"}}}},
	{SignalHiddenStyle, []form{{must(`(?i)style\s*=\s*` +
		`["'][^"']*(?:display\s*:\s*none|visibility\s*:\s*hidden|` +
		`font-size\s*:\s*0)`), []string{"="}}}},
	{SignalRoleLabel, []form{
		{must(`(?im)^\s*(?:system|sistema|assistant|assistente|` +
			`developer)\s*:`), []string{":"}},
		{must(`(?i)<\s*/?\s*(?:system|assistant|developer|` +
			`instructions?)\b[^>]*>`), []string{"<"}},
		{
			must(`(?im)^\s*#{2,}\s*(?:system|instructions?)\b`),
			[]string{"##"},
		},
	}},
	{SignalHTMLComment, []form{
		{must(`<!--[\s\S]*?\pL[\s\S]*?-->`), []string{"<!--"}},
	}},
}

// #endregion

// #region injection-form
// form é uma expressão e os trechos sem os quais ela não casa: todo
// casamento dela contém um deles. strings.Contains é muito mais barato
// que a expressão, e um texto sem nenhum deles é descartado antes dela.
// O teste de fuzzing confere que o descarte nunca muda a resposta.
type form struct {
	re    *regexp.Regexp
	needs []string
}

func (f form) match(text string) bool {
	for _, need := range f.needs {
		if strings.Contains(text, need) {
			return f.re.MatchString(text)
		}
	}
	return false
}

// #endregion

// Actions é a decisão de cada sinal. Num resultado, qualquer coisa que
// não seja allow barra o resultado inteiro.
type Actions map[string]domain.Decision

// #region injection-actions
// DefaultActions barra os sinais fortes e deixa os fracos passar, com
// registro: barrá-los barraria uma em cada três respostas legítimas do
// corpus. O que passa chega ao agente marcado como não confiável.
func DefaultActions() Actions {
	return Actions{
		SignalHidden:        domain.DecisionDeny,
		SignalChatDelimiter: domain.DecisionDeny,
		SignalHiddenStyle:   domain.DecisionDeny,
		SignalRoleLabel:     domain.DecisionAllow,
		SignalHTMLComment:   domain.DecisionAllow,
	}
}

// #endregion

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
			if form.match(text) {
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

// #region injection-hidden-rune
// hiddenRune acha o primeiro caractere que não aparece: formatação
// (Cf) ou controle (Cc) fora de quebra de linha e tabulação. O U+200D
// logo depois de um símbolo é o que junta emojis numa família, e não
// conta.
func hiddenRune(text string) (rune, bool) {
	previous := rune(0)
	for _, r := range text {
		joiner := r == '\u200d' && unicode.Is(unicode.So, previous)
		if !joiner && (unicode.Is(unicode.Cf, r) || (unicode.Is(
			unicode.Cc, r) && !strings.ContainsRune("\n\r\t", r))) {
			return r, true
		}
		previous = r
	}
	return 0, false
}

// #endregion

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
