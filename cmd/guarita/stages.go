package main

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/adapters/dlp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/injection"
	"github.com/aluiziodeveloper/guarita/internal/adapters/schema"
	"github.com/aluiziodeveloper/guarita/internal/adapters/untrusted"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/pipeline"
)

// Nomes das etapas, como o operador os escreve.
const (
	stageSchema     = "schema"
	stageDLP        = "dlp"
	stageInjection  = "injection"
	stageClassifier = "classifier"
	stageMark       = "mark"
)

var (
	errStageConfig  = errors.New("stage needs configuration")
	errPipelineFlag = errors.New(
		"-pipeline must be name=stage,… or stage,…, once per route")
	errMissingStage = errors.New("route lacks a required stage")
)

// #region cmd-guarita-registry
// registry diz como criar cada etapa com a configuração da Guarita.
// Uma etapa nova entra aqui, e em nenhum outro lugar: a montagem, o
// caso de uso e o transporte não mudam.
func registry(
	c inspectConfig, catalog *app.PinCatalog,
	classifier app.ResultInspector,
) pipeline.Registry {
	return pipeline.Registry{
		stageSchema: requires(catalog, "-catalog",
			func(catalog *app.PinCatalog) pipeline.Stage {
				return pipeline.Stage{Call: schema.New(catalog)}
			}),
		stageDLP: always(pipeline.Stage{Call: newDLP(c)}),
		stageInjection: always(pipeline.Stage{
			Result: injection.New(injection.DefaultActions()),
		}),
		stageClassifier: requires(classifier, "-classifier",
			func(classifier app.ResultInspector) pipeline.Stage {
				return pipeline.Stage{Result: classifier}
			}),
		stageMark: always(pipeline.Stage{Marker: untrusted.New()}),
	}
}

// #endregion

// #region cmd-guarita-requires
// requires cria a etapa só se a dependência existir; sem ela, o erro
// diz qual opção falta. T é qualquer tipo comparável com o valor zero,
// como um ponteiro ou uma interface, e o compilador o deduz.
func requires[T comparable](
	dependency T, flag string, stage func(T) pipeline.Stage,
) pipeline.Factory {
	return func() (pipeline.Stage, error) {
		var zero T
		if dependency == zero {
			err := fmt.Errorf("%w: %s", errStageConfig, flag)
			return pipeline.Stage{}, err
		}
		return stage(dependency), nil
	}
}

// always é a fábrica de uma etapa que não depende de nada.
func always(stage pipeline.Stage) pipeline.Factory {
	return func() (pipeline.Stage, error) { return stage, nil }
}

// #endregion

// newDLP cria o inspetor DLP com o limiar e os hosts das opções.
func newDLP(c inspectConfig) *dlp.Inspector {
	var options []dlp.Option
	if hosts := list(c.hosts); len(hosts) > 0 {
		options = append(options, dlp.WithAllowedHosts(hosts...))
	}
	return dlp.New(dlp.DefaultActions(), c.entropy, options...)
}

// #region cmd-guarita-default-stages
// defaultStages traduz as opções dos Capítulos 13 a 15 para a linha de
// todas as rotas, na ordem que elas sempre tiveram: na ida, o esquema,
// a conferência mais barata, antes do DLP; na volta, as regras antes
// do classificador, e a marcação no fim.
func defaultStages(c inspectConfig, hasCatalog bool) []string {
	var names []string
	for _, stage := range []struct {
		on   bool
		name string
	}{
		{c.validation && hasCatalog, stageSchema},
		{c.dlp, stageDLP},
		{c.results, stageInjection},
		{c.classifier.endpoint != "", stageClassifier},
		{c.mark, stageMark},
	} {
		if stage.on {
			names = append(names, stage.name)
		}
	}
	return names
}

// #endregion

// #region cmd-guarita-pipeline-flag
// pipelineFlag lê -pipeline, que pode se repetir: nome=etapas para a
// rota de -server com esse nome; só etapas, para as rotas sem linha
// própria. Uma lista vazia, como em "mail=", é uma rota sem etapas.
func pipelineFlag(c *inspectConfig) func(string) error {
	return func(value string) error {
		route, stages, named := strings.Cut(value, "=")
		if !named {
			route, stages = "*", value
		} else if !validServerName.MatchString(route) {
			return errPipelineFlag
		}
		if _, repeated := c.pipelines[route]; repeated {
			return errPipelineFlag
		}
		if c.pipelines == nil {
			c.pipelines = map[string][]string{}
		}
		c.pipelines[route] = list(stages)
		return nil
	}
}

// #endregion

// #region cmd-guarita-route-line
// routeLine monta a linha de uma rota: a dela, se houver; senão, a de
// -pipeline sem nome; senão, a das opções dos capítulos anteriores. A
// rota sem uma etapa de -require-stages não sobe.
func routeLine(
	c inspectConfig, route string, catalog *app.PinCatalog,
	classifier app.ResultInspector, logger *slog.Logger,
) (pipeline.Pipeline, error) {
	names, ok := c.pipelines[route]
	if !ok || route == "" {
		names, ok = c.pipelines["*"]
	}
	if !ok {
		names = defaultStages(c, catalog != nil)
	}
	line, err := registry(c, catalog, classifier).Build(names)
	if err != nil {
		return line, fmt.Errorf("route %q: %w", route, err)
	}
	if missing := line.Missing(c.required); len(missing) > 0 {
		return line, fmt.Errorf("%w: route %q: %s", errMissingStage,
			route, strings.Join(missing, ","))
	}
	logger.Info("linha de inspetores", "server", route,
		"stages", line.Names)
	return line, nil
}

// #endregion

// checkRoutes recusa -pipeline com o nome de uma rota que não existe:
// o erro de digitação deixaria a rota certa com a linha padrão.
func (c config) checkRoutes() error {
	for route := range c.inspect.pipelines {
		if route == "*" || slices.ContainsFunc(c.servers,
			func(s upstreamServer) bool { return s.name == route }) {
			continue
		}
		return fmt.Errorf("%w: no -server %q", errPipelineFlag, route)
	}
	return nil
}
