package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/aluiziodeveloper/guarita/internal/adapters/audit"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

var errAuditUsage = errors.New(
	"uso: guarita audit verify [-head hash] arquivo")

// auditConfig liga a trilha de auditoria e a exportação dos spans.
type auditConfig struct {
	file, keyFile, traceFile string
}

// #region cmd-guarita-audit-flags
// auditFlags declara a trilha, a chave do HMAC dos argumentos e o
// arquivo dos spans. A chave fica num arquivo, como as credenciais.
func auditFlags(flags *flag.FlagSet, c *auditConfig) {
	flags.StringVar(&c.file, "audit", "",
		"trilha de auditoria encadeada, em JSON Lines")
	flags.StringVar(&c.keyFile, "audit-key", "",
		"arquivo com a chave do HMAC dos argumentos na trilha")
	flags.StringVar(&c.traceFile, "trace-file", "",
		"arquivo em que os spans da Guarita são gravados, em JSON")
}

// #endregion

// #region cmd-guarita-open-audit
// openAudit abre a trilha. Sem -audit, não há auditor, e o retorno é
// a interface nula, e não um *audit.Trail nulo, pela lição do
// Capítulo 18.
func (c auditConfig) openAudit() (app.Auditor, func() error, error) {
	none := func() error { return nil }
	if c.file == "" {
		return nil, none, nil
	}
	var key []byte
	if c.keyFile != "" {
		s, err := secret.ReadFile(c.keyFile)
		if err != nil {
			return nil, none, err
		}
		key = []byte(s.Reveal())
	}
	trail, err := audit.Open(c.file, key)
	if err != nil {
		return nil, none, err
	}
	return trail, trail.Close, nil
}

// #endregion

// #region cmd-guarita-tracer
// tracer cria o provedor de spans do OpenTelemetry, com um exportador
// que escreve cada span em JSON no arquivo, na hora em que ele termina.
// Sem -trace-file, não há tracer, e a Guarita só carrega o traceparent.
func (c auditConfig) tracer() (trace.Tracer, func() error, error) {
	if c.traceFile == "" {
		return nil, func() error { return nil }, nil
	}
	//nolint:gosec // G304: o caminho vem da configuração do operador
	file, err := os.OpenFile(c.traceFile,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(file))
	if err != nil {
		return nil, nil, errors.Join(err, file.Close())
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter))
	return provider.Tracer("guarita"), func() error {
		return errors.Join(provider.Shutdown(context.Background()),
			file.Close())
	}, nil
}

// #endregion

// #region cmd-guarita-audit-verify
// runAudit é guarita audit verify: diz onde a trilha quebra, se
// quebrar, e confere a última entrada contra -head, o hash guardado à
// parte. Sem -head, a remoção das últimas linhas não aparece.
func runAudit(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	head := flags.String("head", "", "hash esperado da última entrada")
	if len(args) == 0 || args[0] != "verify" ||
		flags.Parse(args[1:]) != nil || flags.NArg() != 1 {
		return errAuditUsage
	}
	report, err := verifyFile(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("trilha quebrada: %w", err)
	}
	last := report.Last
	if *head != "" && last.Hash != *head {
		return fmt.Errorf("trilha truncada ou trocada: termina na %d, "+
			"com hash %.16s…", last.Seq, last.Hash)
	}
	_, _ = fmt.Fprintf(stdout, "trilha íntegra: %d entradas; "+
		"cabeça %s\n", report.Entries, last.Hash)
	return nil
}

// #endregion

// verifyFile abre o arquivo da trilha e o verifica.
func verifyFile(path string) (audit.Report, error) {
	//nolint:gosec // G304: o caminho vem de quem investiga
	file, err := os.Open(path)
	if err != nil {
		return audit.Report{}, err
	}
	defer func() { _ = file.Close() }()
	return audit.Verify(file)
}
