// Package scanner procura, nas definições de ferramenta, sinais de texto
// disfarçado: caracteres que o operador não vê e o modelo lê. É uma
// varredura estrutural; ela não tenta entender o que o texto diz.
package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Sinais que a varredura relata.
const (
	SignalName          = "name"          // nome fora do conjunto da especificação
	SignalFormat        = "format"        // caractere de formatação (Cf)
	SignalControl       = "control"       // caractere de controle (Cc)
	SignalCompatibility = "compatibility" // texto que muda sob NFKC
	SignalMalformed     = "malformed"     // definição que não é JSON válido
)

// validName é o conjunto de nomes que a especificação recomenda: letras
// e dígitos ASCII, sublinhado, hífen e ponto, de 1 a 128 caracteres.
var validName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// allowed são os controles legítimos numa descrição.
const allowed = "\n\r\t"

// Structural é a varredura estrutural.
type Structural struct{}

// #region scanner-scan
// Scan percorre todas as strings da definição, chaves e valores, em
// qualquer profundidade: uma instrução pode estar na descrição de um
// parâmetro tanto quanto na da ferramenta.
func (Structural) Scan(
	definition domain.ToolDefinition,
) []domain.Finding {
	var findings []domain.Finding
	if !validName.MatchString(definition.Name) {
		findings = append(findings, domain.Finding{
			Field:  "name",
			Signal: SignalName,
			Detail: fmt.Sprintf("%q", definition.Name),
		})
	}
	decoder := json.NewDecoder(bytes.NewReader(definition.Raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return append(findings, domain.Finding{
			Signal: SignalMalformed,
			Detail: err.Error(),
		})
	}
	walk("", value, func(field, text string) {
		findings = append(findings, inspect(field, text)...)
	})
	return findings
}

// #endregion

// #region scanner-inspect
// inspect relata o primeiro caractere suspeito de cada tipo num texto e
// o texto que muda sob NFKC.
func inspect(field, text string) []domain.Finding {
	findings := characters(field, text)
	// NFC contra NFKC: só conta a diferença de compatibilidade (largura
	// total, ligaduras). Um acento decomposto muda sob as duas.
	if canonical := norm.NFC.String(text); canonical !=
		norm.NFKC.String(canonical) {
		findings = append(findings, domain.Finding{
			Field: field, Signal: SignalCompatibility,
			Detail: "muda sob NFKC",
		})
	}
	return findings
}

// #endregion

// #region scanner-characters
// characters anda de runa em runa: range sobre uma string decodifica o
// UTF-8. Quebra de linha e tabulação são controles legítimos.
func characters(field, text string) []domain.Finding {
	var findings []domain.Finding
	seen := map[string]bool{}
	for _, r := range text {
		signal := ""
		switch {
		case unicode.Is(unicode.Cf, r):
			signal = SignalFormat
		case unicode.Is(unicode.Cc, r) &&
			!strings.ContainsRune(allowed, r):
			signal = SignalControl
		}
		if signal != "" && !seen[signal] {
			seen[signal] = true
			findings = append(findings, domain.Finding{
				Field:  field,
				Signal: signal, Detail: fmt.Sprintf("U+%04X", r),
			})
		}
	}
	return findings
}

// #endregion

// walk chama visit para cada string do valor, com o caminho dela. As
// chaves de objeto também são texto que chega ao modelo.
func walk(path string, value any, visit func(field, text string)) {
	switch v := value.(type) {
	case string:
		visit(path, v)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys) // ordem estável nos relatórios
		for _, key := range keys {
			field := strings.TrimPrefix(path+"."+key, ".")
			visit(field+" (chave)", key)
			walk(field, v[key], visit)
		}
	case []any:
		for i, item := range v {
			walk(fmt.Sprintf("%s[%d]", path, i), item, visit)
		}
	}
}
