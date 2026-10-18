package main

import (
	"os"
	"path/filepath"
	"testing"
)

// lab monta a raiz do laboratório: um relatório, um symlink interno e
// três caminhos para o segredo ao lado da raiz.
func lab(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "dados")
	secret := filepath.Join(dir, "segredo.txt")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(root, 0o750))
	must(os.WriteFile(filepath.Join(root, "relatorio.txt"),
		[]byte("Relatório trimestral."), 0o600))
	must(os.WriteFile(secret, []byte("[LAB] segredo do host"), 0o600))
	must(os.Symlink("relatorio.txt", filepath.Join(root, "interno.txt")))
	must(os.Symlink(secret, filepath.Join(root, "absoluto.txt")))
	must(os.Symlink("../segredo.txt", filepath.Join(root, "relativo.txt")))
	return root
}

// #region cmd-lab-files-test
// Os dois modos diante dos mesmos nomes: o ingênuo entrega o segredo
// pelos dois symlinks; os.Root só entrega o que está dentro da raiz.
func TestReaders(t *testing.T) {
	root := lab(t)
	safe, closeRoot, err := rooted(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeRoot() }()
	for _, name := range []string{
		"relatorio.txt", "interno.txt", "../segredo.txt",
		"absoluto.txt", "relativo.txt",
	} {
		_, naiveErr := naive(root)(name)
		data, rootErr := safe(name)
		t.Logf("%-15s ingênuo=%-5v os.Root=%q %v", name,
			naiveErr == nil, data, rootErr)
		if (rootErr == nil) != (name == "relatorio.txt" ||
			name == "interno.txt") {
			t.Errorf("%s: os.Root mudou de comportamento", name)
		}
	}
}

// #endregion
