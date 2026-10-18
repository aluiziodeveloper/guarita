package naivefiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// #region examples-naivefiles-test
// A raiz tem um relatório e um symlink para um arquivo ao lado dela. O
// "..", escrito, é recusado; o symlink entrega o arquivo de fora.
func TestRead_symlink(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "dados")
	secret := filepath.Join(dir, "segredo.txt")
	_ = os.Mkdir(root, 0o750)
	_ = os.WriteFile(filepath.Join(root, "relatorio.txt"),
		[]byte("Relatório trimestral."), 0o600)
	_ = os.WriteFile(secret, []byte("[LAB] segredo do host"), 0o600)
	link := filepath.Join(root, "atalho.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"relatorio.txt", "../segredo.txt", "atalho.txt",
	} {
		data, err := Read(root, name)
		t.Logf("%-15s %q %v", name, data, err)
		if name == "../segredo.txt" && !errors.Is(err, ErrOutside) {
			t.Error("o .. escrito passou")
		}
		if name == "atalho.txt" && err != nil {
			t.Error("o ingênuo mudou de comportamento")
		}
	}
}

// #endregion
