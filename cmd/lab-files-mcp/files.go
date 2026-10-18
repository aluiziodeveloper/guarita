package main

import (
	"os"

	"github.com/aluiziodeveloper/guarita/examples/naivefiles"
)

// reader lê um arquivo pelo nome que o agente mandou.
type reader func(name string) ([]byte, error)

// naive é a leitura do exemplo ingênuo: prefixo conferido, symlink
// seguido.
func naive(dir string) reader {
	return func(name string) ([]byte, error) {
		return naivefiles.Read(dir, name)
	}
}

// #region cmd-lab-files-rooted
// rooted abre a raiz uma vez, com os.OpenRoot, e toda leitura passa
// por ela. Um componente que sai da raiz, um ".." que escapa ou um
// symlink para fora, relativo ou absoluto, é erro, e não um arquivo.
// Um symlink que fica dentro da raiz continua valendo.
func rooted(dir string) (reader, func() error, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	return root.ReadFile, root.Close, nil
}

// #endregion
