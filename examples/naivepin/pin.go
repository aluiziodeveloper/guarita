// Package naivepin mostra as duas fixações ingênuas do Capítulo 9: a da
// descrição, que não vê o resto da definição, e a dos bytes, que vê
// mudança onde só a forma mudou.
package naivepin

import (
	"bytes"
	"encoding/json"
)

// #region examples-naivepin
// SameDescription compara só a descrição: é o texto que parece ser a
// instrução ao modelo.
func SameDescription(pinned, current []byte) bool {
	var a, b struct {
		Description string `json:"description"`
	}
	return json.Unmarshal(pinned, &a) == nil &&
		json.Unmarshal(current, &b) == nil &&
		a.Description == b.Description
}

// SameBytes compara a definição como chegou, byte a byte.
func SameBytes(pinned, current []byte) bool {
	return bytes.Equal(bytes.TrimSpace(pinned),
		bytes.TrimSpace(current))
}

// #endregion
