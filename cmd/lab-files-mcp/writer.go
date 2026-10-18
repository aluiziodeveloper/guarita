package main

import (
	"errors"
	"os"
)

// #region cmd-lab-files-writer
// rootedWriter escreve arquivos novos dentro de dir, por os.Root.
// O_CREATE com O_EXCL só cria: um arquivo que já existe é ErrExist, e
// um symlink no lugar do nome também, porque O_EXCL não segue o link
// final. A raiz recusa o caminho que sai dela.
func rootedWriter(dir string) (
	func(name string, data []byte) error, func() error, error,
) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	write := func(name string, data []byte) error {
		f, err := root.OpenFile(name,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return errors.Join(err, f.Close())
	}
	return write, root.Close, nil
}

// #endregion
