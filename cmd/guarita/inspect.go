package main

import (
	"flag"
	"log/slog"

	"github.com/aluiziodeveloper/guarita/internal/adapters/dlp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/injection"
	"github.com/aluiziodeveloper/guarita/internal/adapters/schema"
	"github.com/aluiziodeveloper/guarita/internal/adapters/untrusted"
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

// #region cmd-guarita-result-options
// resultOptions monta a revisão do caminho de volta. As regras vêm
// antes do classificador: custam microssegundos, e um sinal forte
// barra o resultado sem perguntar ao modelo. Sem nenhuma das opções,
// a Guarita nem lê as respostas de tools/call.
func resultOptions(
	c inspectConfig, classifier app.ResultInspector,
) []app.Option {
	var options []app.Option
	if c.results {
		options = append(options, app.WithResultInspectors(
			injection.New(injection.DefaultActions())))
	}
	if classifier != nil {
		options = append(options, app.WithResultInspectors(classifier))
	}
	if c.mark {
		options = append(options, app.WithResultMarker(untrusted.New()))
	}
	return options
}

// #endregion

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
