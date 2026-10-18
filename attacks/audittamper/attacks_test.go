package audittamper

import (
	"slices"
	"strings"
	"testing"
)

type tamper struct {
	name   string
	change func([]string) []string
}

// #region attacks-audittamper-cases
// edits são as adulterações que mexem em linhas sem refazer a cadeia
// inteira: a cadeia sozinha as acusa.
var edits = []tamper{
	{"decisão trocada", func(l []string) []string {
		l[1] = strings.Replace(l[1], `"deny"`, `"allow"`, 1)
		return l
	}},
	{"decisão trocada e hash refeito", func(l []string) []string {
		l[1] = strings.Replace(l[1], `"deny"`, `"allow"`, 1)
		return append(reseal(l[:2], 1), l[2:]...)
	}},
	{"linha apagada", func(l []string) []string {
		return slices.Delete(l, 1, 2)
	}},
	{"linhas trocadas", func(l []string) []string {
		l[1], l[2] = l[2], l[1]
		return l
	}},
	{"campo a mais", func(l []string) []string {
		l[3] = strings.Replace(l[3], `{"seq"`, `{"ok":true,"seq"`, 1)
		return l
	}},
}

// #endregion

// #region attacks-audittamper-rewrites
// rewrites deixam uma cadeia que fecha: quem conhece o algoritmo
// refaz os hashes depois da troca, ou corta o fim. Só a cabeça,
// guardada fora do alcance de quem adulterou, as acusa.
var rewrites = []tamper{
	{"cadeia refeita do meio", func(l []string) []string {
		l[1] = strings.Replace(l[1], `"deny"`, `"allow"`, 1)
		return reseal(l, 1)
	}},
	{"fim cortado", func(l []string) []string { return l[:3] }},
}

// #endregion

// #region attacks-audittamper-test
// Cada adulteração sobre uma trilha de cinco entradas, conferida pela
// cadeia sozinha e pela cadeia com a cabeça guardada à parte. Só a
// cabeça acusa a cadeia refeita e o fim cortado.
func TestAdulteracao(t *testing.T) {
	for _, c := range append(slices.Clone(edits), rewrites...) {
		lines, head := record(t)
		tampered := c.change(slices.Clone(lines))
		chain := verify(tampered, "")
		anchored := verify(tampered, head)
		t.Logf("%-31s cadeia: %-26s com cabeça: %s", c.name, chain,
			anchored)
		if anchored == "passou" {
			t.Errorf("%s passou até pela cabeça", c.name)
		}
	}
	if lines, head := record(t); verify(lines, head) != "passou" {
		t.Error("a trilha intacta foi acusada")
	}
}

// #endregion
