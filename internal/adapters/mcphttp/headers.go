package mcphttp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
)

// mirroredSince é a primeira revisão que exige os cabeçalhos espelhados.
// As datas das revisões se comparam como texto.
const mirroredSince = "2026-07-28"

// codeHeaderMismatch é o código JSON-RPC da revisão 2026-07-28 para
// cabeçalho ausente, malformado ou diferente do corpo.
const codeHeaderMismatch = -32020

// ErrHeaderMismatch recusa um pedido cujo cabeçalho diverge do corpo.
var ErrHeaderMismatch = errors.New("header mismatch")

// #region mcphttp-check-mirrored
// checkMirrored confere cada cabeçalho espelhado contra o corpo, nas
// revisões que os exigem. Repetido, ausente ou diferente: recusa.
func checkMirrored(header http.Header, message mcpproxy.Message) error {
	versions := header.Values("MCP-Protocol-Version")
	if len(versions) == 0 || (len(versions) == 1 &&
		versions[0] < mirroredSince) {
		return nil // revisão antiga: não há o que conferir
	}
	body := message.Mirrored()
	if err := same(header, "MCP-Protocol-Version", body.Version,
		false); err != nil {
		return err
	}
	if err := same(header, "Mcp-Method", message.Method,
		false); err != nil {
		return err
	}
	if !body.HasName {
		return nil
	}
	return same(header, "Mcp-Name", body.Name, true)
}

// #endregion

// same exige o cabeçalho uma vez só e igual ao corpo. Dois valores são
// duas leituras possíveis e contam como divergência.
func same(header http.Header, name, body string, encoded bool) error {
	values := header.Values(name)
	if len(values) != 1 {
		return mismatch(name, strings.Join(values, ", "), body)
	}
	value, ok := values[0], true
	if encoded {
		value, ok = decodeName(values[0])
	}
	if !ok || value != body {
		return mismatch(name, values[0], body)
	}
	return nil
}

// decodeName desfaz o formato sentinela =?base64?…?= da especificação.
func decodeName(value string) (string, bool) {
	encoded, ok := strings.CutPrefix(value, "=?base64?")
	if !ok {
		return value, value != ""
	}
	encoded, ok = strings.CutSuffix(encoded, "?=")
	if !ok {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	return string(decoded), err == nil && len(decoded) > 0
}

func mismatch(name, header, body string) error {
	return fmt.Errorf("%w: %s header %q does not match body %q",
		ErrHeaderMismatch, name, header, body)
}
