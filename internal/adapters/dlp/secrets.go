package dlp

import (
	"encoding/base64"
	"math"
	"regexp"
	"unicode/utf8"
)

// #region dlp-secret-patterns
// secretPatterns são formatos públicos de credenciais: o prefixo diz o
// emissor e o tipo. Um casamento aqui é quase certamente uma chave.
var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"aws-access-key", must(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"github-token", must(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`)},
	{"slack-token", must(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"private-key", must(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
}

// must compila na inicialização: um padrão errado derruba a partida.
var must = regexp.MustCompile

// #endregion

// token é uma sequência longa no alfabeto do base64 e dos hexadecimais.
var token = regexp.MustCompile(`[A-Za-z0-9+/=_-]{12,}`)

// #region dlp-entropy
// entropy é a entropia de Shannon do texto, em bits por caractere: a
// média de -log2(p) sobre a frequência p de cada símbolo. Texto comum
// fica perto de 4; uma chave aleatória em base64, perto de 6.
func entropy(text string) float64 {
	counts := map[rune]int{}
	for _, r := range text {
		counts[r]++
	}
	total := float64(utf8.RuneCountInString(text))
	bits := 0.0
	for _, n := range counts {
		p := float64(n) / total
		bits -= p * math.Log2(p)
	}
	return bits
}

// #endregion

// #region dlp-decode
// decoded tenta ler o token como base64, padrão ou de URL. Só devolve
// texto: um binário decodificado não tem CPF nem chave para achar.
func decoded(candidate string) (string, bool) {
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding, base64.URLEncoding,
		base64.RawStdEncoding, base64.RawURLEncoding,
	} {
		raw, err := encoding.DecodeString(candidate)
		if err == nil && utf8.Valid(raw) && printable(raw) {
			return string(raw), true
		}
	}
	return "", false
}

// #endregion

func printable(raw []byte) bool {
	for _, b := range raw {
		if b < ' ' && b != '\n' && b != '\t' {
			return false
		}
	}
	return len(raw) > 0
}
