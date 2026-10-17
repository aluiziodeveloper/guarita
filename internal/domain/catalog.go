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
