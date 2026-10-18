package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
)

// version é a versão da release. O GoReleaser a grava no binário com
// -ldflags "-X main.version=…"; num go build comum, ela fica "dev".
var version = "dev"

var errNoBuildInfo = errors.New("binário sem informação de build")

// #region cmd-guarita-version
// runVersion diz de onde o binário veio, pelo que o próprio go build
// gravou nele: o módulo, a versão do Go, o commit, se a cópia tinha
// mudanças locais, as opções de build e, com -deps, cada dependência
// com a versão e o hash do go.sum.
func runVersion(
	_ context.Context, args []string, out, stderr io.Writer,
) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	deps := flags.Bool("deps", false, "lista as dependências")
	if err := flags.Parse(args); err != nil {
		return err
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errNoBuildInfo
	}
	return printBuild(out, info, *deps)
}

// #endregion

// #region cmd-guarita-print-build
// printBuild escreve a informação de build; -deps é o mesmo que
// go version -m mostra de um binário.
func printBuild(out io.Writer, info *debug.BuildInfo, deps bool) error {
	_, _ = fmt.Fprintf(out, "guarita %s\n%s %s %s\n", version,
		info.Main.Path, info.Main.Version, info.GoVersion)
	for _, s := range info.Settings {
		if wanted[s.Key] {
			_, _ = fmt.Fprintf(out, "  %s=%s\n", s.Key, s.Value)
		}
	}
	for _, d := range info.Deps {
		if deps {
			_, _ = fmt.Fprintf(out, "  dep %s %s %s\n", d.Path,
				d.Version, d.Sum)
		}
	}
	_, err := fmt.Fprintf(out, "  %d dependências\n", len(info.Deps))
	return err
}

// #endregion

// wanted são as opções de build que dizem de onde o binário veio. O
// go build não grava -ldflags quando há -trimpath, porque elas podem
// ter caminhos da máquina.
var wanted = map[string]bool{
	"-trimpath": true, "CGO_ENABLED": true,
	"vcs.revision": true, "vcs.time": true, "vcs.modified": true,
}
