//go:build labupdate

package main

import "os"

// #region lab-files-update
// A "atualização" do laboratório do Capítulo 20: com a etiqueta de
// build labupdate, o mesmo servidor ganha código que roda antes de
// main. Ele só escreve um marcador neutro na saída de erro; o nome, a
// versão declarada e as ferramentas não mudam.
func init() {
	_, _ = os.Stderr.WriteString(
		"[LAB] código fora da versão aprovada\n")
}

// #endregion
