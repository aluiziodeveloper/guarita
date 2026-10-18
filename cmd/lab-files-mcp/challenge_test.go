package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Desafio do Capítulo 17: o servidor de arquivos ganha a escrita.
// rootedWriter(dir) devolve uma função que escreve um arquivo novo
// dentro de dir, por os.Root, e a que fecha a raiz. Ela recusa o
// caminho que sai da raiz, o arquivo que já existe, para não
// sobrescrever, e o symlink no lugar do arquivo, mesmo que aponte para
// dentro. O pacote não compila até existir rootedWriter.
func TestRootedWriter_desafio(t *testing.T) {
	root := lab(t)
	write, closeRoot, err := rootedWriter(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeRoot() }()
	for _, c := range []struct {
		name string
		ok   bool
	}{
		{"novo.txt", true},
		{"../fora.txt", false},
		{"relatorio.txt", false}, // já existe
		{"interno.txt", false},   // symlink para dentro
		{"absoluto.txt", false},  // symlink para fora
	} {
		err := write(c.name, []byte("[LAB] escrito"))
		t.Logf("%-14s %v", c.name, err)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root),
		"fora.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("o arquivo de fora foi criado")
	}
}
