package naiveaudit

import (
	"bytes"
	"strings"
	"testing"
)

// #region examples-naiveaudit-test
// Duas chamadas registradas; a negação é trocada por uma permissão com
// um editor de texto qualquer, e o registro continua um JSON válido,
// que ninguém tem como saber que mudou. O CPF de exemplo está nele.
func TestLogger_editavel(t *testing.T) {
	var log bytes.Buffer
	record := Logger(&log)
	cpf := "123456789" + "09" // o CPF de exemplo das documentações
	record("relatorios", "web_search", "allow", `{"q":"trimestre"}`)
	record("relatorios", "send_email", "deny", `{"body":"`+cpf+`"}`)
	edited := strings.Replace(log.String(), `"decision":"deny"`,
		`"decision":"allow"`, 1)
	t.Logf("depois da edição:\n%s", edited)
	if strings.Contains(edited, `"deny"`) ||
		!strings.Contains(edited, cpf) {
		t.Error("o ingênuo mudou de comportamento")
	}
}

// #endregion
