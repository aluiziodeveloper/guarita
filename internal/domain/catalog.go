package domain

import "encoding/json"

// #region domain-pin
// Pin é a definição fixada de uma ferramenta num servidor. Quando o
// servidor anuncia outra, Changed passa a valer, e a definição vista
// fica guardada para o operador revisar.
type Pin struct {
	Server      string
	Tool        string
	Fingerprint Fingerprint
	Definition  json.RawMessage

	Changed         bool
	SeenFingerprint Fingerprint
	SeenDefinition  json.RawMessage
}

// #endregion

// #region domain-finding
// Finding é um sinal suspeito numa definição de ferramenta: em que
// campo, qual sinal e o detalhe que o operador precisa para conferir.
type Finding struct {
	Field  string // caminho no JSON, como inputSchema.properties.path
	Signal string
	Detail string
}

// #endregion

// #region domain-skeleton
// Skeleton é a silhueta de um nome de ferramenta: minúsculas ASCII, com
// hífen e ponto trocados por sublinhado. read_file, Read-File e
// read.file têm a mesma silhueta, e um modelo os confunde.
//
// Só o ASCII muda, como na função lower do SQLite: a migração 0003
// calcula a mesma silhueta em SQL para os pins que já existiam.
func Skeleton(name string) string {
	out := []byte(name)
	for i, b := range out {
		switch {
		case 'A' <= b && b <= 'Z':
			out[i] = b + ('a' - 'A')
		case b == '-' || b == '.':
			out[i] = '_'
		}
	}
	return string(out)
}

// #endregion
