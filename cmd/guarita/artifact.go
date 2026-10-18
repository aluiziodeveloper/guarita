package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aluiziodeveloper/guarita/internal/adapters/artifact"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

var errDigestUsage = errors.New("uso: guarita digest <comando ou arquivo>")

// #region cmd-guarita-digest-flag
// artifactFlags declara o digest aprovado do servidor stdio: o do
// executável, no host, ou o da imagem, com -sandbox.
func artifactFlags(flags *flag.FlagSet, c *config) {
	flags.Func("server-digest", "digest aprovado do servidor stdio "+
		"(sha256:…)", func(value string) error {
		d, err := domain.ParseDigest(value)
		c.digest = d
		return err
	})
}

// #endregion

// #region cmd-guarita-verified
// verified confere o servidor antes de iniciá-lo. No host, o comando
// volta com o caminho conferido no lugar do nome, e é esse arquivo que
// o sistema executa. Na sandbox, quem confere os bytes é o Docker, pelo
// digest da referência da imagem. Sem -server-digest, nada muda.
func (s stdioServer) verified() ([]string, error) {
	if s.digest == "" {
		return s.command, nil
	}
	if s.sandbox.image != "" {
		return s.command, artifact.CheckImage(s.sandbox.image, s.digest)
	}
	path, err := artifact.Resolve(s.command[0])
	if err != nil {
		return nil, err
	}
	if err := artifact.Check(path, s.digest); err != nil {
		return nil, err
	}
	return append([]string{path}, s.command[1:]...), nil
}

// #endregion

// #region cmd-guarita-digest
// runDigest imprime o digest de um executável, para o operador aprovar
// a versão que examinou: guarita digest lab-files.
func runDigest(
	_ context.Context, args []string, out, _ io.Writer,
) error {
	if len(args) != 1 {
		return errDigestUsage
	}
	path, err := artifact.Resolve(args[0])
	if err != nil {
		return err
	}
	//nolint:gosec // G304: o caminho vem do operador
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	d, err := domain.DigestOf(file)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s  %s\n", d, path)
	return err
}

// #endregion
