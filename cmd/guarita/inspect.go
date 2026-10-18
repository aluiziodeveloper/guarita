package main

import (
	"flag"
	"log/slog"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// inspectConfig liga os inspetores do Capítulo 13.
type inspectConfig struct {
	dlp        bool
	entropy    float64
	hosts      string // separados por vírgula; vazio não confere destinos
	validation bool   // argumentos contra o inputSchema; exige -catalog
	results    bool   // revisa os resultados de tools/call
	mark       bool   // marca os resultados como não confiáveis
	classifier classifierConfig
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
	flags.BoolVar(&c.results, "inspect-results", false,
		"barra resultados com texto oculto ou delimitadores de conversa")
	flags.BoolVar(&c.mark, "mark-untrusted", false,
		"marca o texto dos resultados como conteúdo não confiável")
	classifierFlags(flags, &c.classifier)
}

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
