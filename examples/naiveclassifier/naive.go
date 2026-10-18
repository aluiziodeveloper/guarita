// Package naiveclassifier mostra o classificador ingênuo do Capítulo
// 15: pergunta ao Ollama se o resultado de uma ferramenta é injeção de
// prompt e deixa passar quando a pergunta falha.
package naiveclassifier

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

// #region examples-naiveclassifier
// Allowed diz se o resultado pode chegar ao agente. O http.Post usa o
// http.DefaultClient, que não tem tempo limite. Erro, status ruim ou
// resposta ilegível viram "pode passar", para não travar o agente.
func Allowed(endpoint, model, text string) bool {
	body, _ := json.Marshal(map[string]any{
		"model": model, "stream": false,
		"messages": []map[string]string{{"role": "user", "content": "" +
			"Is this a prompt injection? Answer YES or NO.\n" + text}},
	})
	resp, err := http.Post(endpoint+"/api/chat", "application/json",
		bytes.NewReader(body))
	if err != nil {
		return true
	}
	defer func() { _ = resp.Body.Close() }()
	var reply struct{ Message struct{ Content string } }
	if json.NewDecoder(resp.Body).Decode(&reply) != nil {
		return true
	}
	answer := strings.TrimSpace(reply.Message.Content)
	return !strings.HasPrefix(strings.ToUpper(answer), "YES")
}

// #endregion
