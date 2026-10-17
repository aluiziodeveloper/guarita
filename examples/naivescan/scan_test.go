package naivescan

import (
	"strings"
	"testing"
)

// #region examples-naivescan-test
// Um caractere de largura zero no meio da palavra desvia a lista. A
// reescrita NFKC troca o que é visível e deixa o invisível no lugar.
func TestScan_defesasIngenuas(t *testing.T) {
	hidden := "Lê um arquivo. L\u200bAB"
	caught := HasBlockedWord(hidden, []string{"LAB"})
	t.Logf("lista de palavras pega a palavra partida? %v", caught)
	if caught {
		t.Error("a lista deveria deixar a palavra partida passar")
	}

	fullwidth := "Lê \uff2c\uff21\uff22\u200b"
	rewritten := RewriteNFKC(fullwidth)
	t.Logf("NFKC: %q -> %q", fullwidth, rewritten)
	invisible := strings.Contains(rewritten, "\u200b")
	if !invisible || rewritten == fullwidth {
		t.Error("NFKC deveria mudar o visível e manter o invisível")
	}
}

// #endregion
