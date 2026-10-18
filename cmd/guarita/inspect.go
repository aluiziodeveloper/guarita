package main

import (
	"flag"
	"log/slog"

	"github.com/aluiziodeveloper/guarita/internal/adapters/dlp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/schema"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// inspectConfig liga os inspetores do Capítulo 13.
type inspectConfig struct {
	dlp        bool
	entropy    float64
	hosts      string // separados por vírgula; vazio não confere destinos
	validation bool   // argumentos contra o inputSchema; exige -catalog
}

// inspectFlags declara as opções dos inspetores.
func inspectFlags(flags *flag.FlagSet, c *inspectConfig) {
	flags.BoolVar(&c.dlp, "dlp", false,
		"inspeciona os argumentos: CPF, chaves, entropia, destinos")
	flags.Float64Var(&c.entropy, "dlp-entropy", 4.3,
		"bits por caractere a partir dos quais um token é segredo")
	flags.StringVar(&c.hosts, "allow-hosts", "",
		"hosts permitidos nas URLs dos argumentos (.dominio vale subdomínios)")
	flags.BoolVar(&c.validation, "validate-arguments", false,
		"valida os argumentos contra o inputSchema fixado no catálogo")
}

// #region cmd-guarita-inspectors
// inspectors monta os inspetores de uma rota. O esquema vem antes: é a
// conferência mais barata e a que recusa a forma inesperada sem olhar o
// conteúdo. Cada inspeção com achados vai para o registro.
func inspectors(
	c inspectConfig, catalog *app.PinCatalog, logger *slog.Logger,
) []app.Option {
	var chosen []app.Inspector
	if c.validation && catalog != nil {
		chosen = append(chosen, schema.New(catalog))
	}
	if c.dlp {
		var options []dlp.Option
		if hosts := list(c.hosts); len(hosts) > 0 {
			options = append(options, dlp.WithAllowedHosts(hosts...))
		}
		chosen = append(chosen, dlp.New(dlp.DefaultActions(), c.entropy,
			options...))
	}
	return []app.Option{
		app.WithInspectors(chosen...),
		app.WithFindings(logFindings(logger)),
	}
}

// #endregion

// logFindings registra o que o inspetor achou: campo e sinal, sem os
// valores, que o inspetor nunca põe nos achados.
func logFindings(logger *slog.Logger) app.FindingsObserver {
	return func(input domain.PolicyInput, inspection domain.Inspection) {
		logger.Warn("argumentos inspecionados", "agent", input.Agent,
			"server", input.Server, "tool", input.Call.ToolName,
			"decision", string(inspection.Decision),
			"findings", summarize(inspection.Findings))
	}
}

func summarize(findings []domain.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Field+":"+f.Signal+":"+f.Detail)
	}
	return out
}
