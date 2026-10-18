package schema

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// pinned é um catálogo de uma ferramenta só.
type pinned map[string]string

func (p pinned) Pinned(_ context.Context, tool string) (json.RawMessage,
	error,
) {
	if definition, ok := p[tool]; ok {
		return json.RawMessage(definition), nil
	}
	return nil, errors.New("not in catalog")
}

var catalog = pinned{
	"http_request": `{"name":"http_request","inputSchema":{
		"type":"object","additionalProperties":false,"required":["url"],
		"properties":{"url":{"type":"string","maxLength":200},
		"method":{"enum":["GET","POST"]},"body":{"type":"string"}}}}`,
	"remote": `{"name":"remote","inputSchema":{
		"$ref":"https://schemas.evil.test/s.json"}}`,
}

// check valida os argumentos de uma ferramenta do catálogo.
func check(
	i *Inspector, tool, arguments string,
) (domain.Inspection, error) {
	return i.Inspect(context.Background(), domain.PolicyInput{
		Call: domain.Call{
			ToolName:  tool,
			Arguments: json.RawMessage(arguments),
		},
	})
}

// #region schema-test
// Os argumentos que o esquema fixado não prevê são negados, e o
// relatório diz onde e por qual regra, sem os valores.
func TestInspect_esquemaFixado(t *testing.T) {
	inspector := New(catalog)
	const url = `"url":"https://lab.test/r"`
	for _, c := range []struct{ name, arguments string }{
		{"conforme", `{` + url + `,"method":"POST"}`},
		{"campo a mais", `{` + url + `,"exfil":"x"}`},
		{"método fora", `{` + url + `,"method":"SEGREDO"}`},
		{"sem url", `{"body":"x"}`},
	} {
		got, err := check(inspector, "http_request", c.arguments)
		t.Logf("%-13s %-5s %v %v", c.name, got.Decision,
			got.Findings, err)
		clean := got.Decision == domain.DecisionAllow
		if clean != (c.name == "conforme") {
			t.Errorf("%s: %s", c.name, got.Decision)
		}
		for _, f := range got.Findings {
			if strings.Contains(f.Detail, "SEGREDO") ||
				strings.Contains(f.Detail, "exfil") {
				t.Errorf("%s: o detalhe vazou o valor", c.name)
			}
		}
	}
	_, err := check(inspector, "remote", `{}`)
	t.Logf("$ref remoto: %v", err)
}

// #endregion

func TestInspect_refRemotoFalha(t *testing.T) {
	if _, err := check(New(catalog), "remote", `{}`); err == nil {
		t.Fatal("um $ref remoto não pode ser resolvido")
	}
}
