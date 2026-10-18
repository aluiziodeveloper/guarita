package injection

import "regexp"

// SignalAltText é texto para o modelo escondido no texto alternativo
// de uma imagem, que a página não mostra.
const SignalAltText = "alt-text"

// altTexts acham o texto alternativo em Markdown, ![texto](endereço),
// com até um nível de colchetes dentro, e os atributos alt e title do
// HTML.
var altTexts = []*regexp.Regexp{
	must(`!\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\(`), // um nível de [ ]
	must(`(?i)\b(?:alt|title)\s*=\s*"([^"]*)"`),
	must(`(?i)\b(?:alt|title)\s*=\s*'([^']*)'`),
}

// altText diz se algum texto alternativo do texto tem um rótulo de
// papel ou um delimitador de conversa. Num texto que ninguém vê, mesmo
// o sinal fraco é forte.
func altText(text string) bool {
	for _, alt := range altTexts {
		for _, match := range alt.FindAllStringSubmatch(text, -1) {
			for _, group := range patterns {
				if group.signal != SignalRoleLabel &&
					group.signal != SignalChatDelimiter {
					continue
				}
				for _, form := range group.forms {
					if form.MatchString(match[1]) {
						return true
					}
				}
			}
		}
	}
	return false
}
