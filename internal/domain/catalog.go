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
