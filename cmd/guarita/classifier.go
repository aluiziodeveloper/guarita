package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/classifier"
	"github.com/aluiziodeveloper/guarita/internal/adapters/injection"
	"github.com/aluiziodeveloper/guarita/internal/adapters/ollama"
	"github.com/aluiziodeveloper/guarita/internal/app"
)

var errClassifier = errors.New("classifier: invalid configuration")

// classifierConfig liga o classificador local do Capítulo 15.
type classifierConfig struct {
	endpoint  string // vazio desliga o classificador
	model     string
	threshold float64
	timeout   time.Duration
	onError   string // deny ou rules
}

// #region cmd-guarita-classifier-flags
// classifierFlags declara as opções do classificador. O limiar 0,8 e
// o prazo de 5 segundos saem das medidas do Capítulo 15, com o modelo
// padrão; outro modelo pede outra medida.
func classifierFlags(flags *flag.FlagSet, c *classifierConfig) {
	flags.StringVar(&c.endpoint, "classifier", "",
		"Ollama que classifica, como http://127.0.0.1:11434")
	flags.StringVar(&c.model, "classifier-model", "qwen2.5:7b",
		"modelo do Ollama que classifica os resultados")
	flags.Float64Var(&c.threshold, "classifier-threshold", 0.8,
		"probabilidade de injeção que barra o resultado")
	flags.DurationVar(&c.timeout, "classifier-timeout", 5*time.Second,
		"prazo do classificador para cada resultado")
	flags.StringVar(&c.onError, "classifier-on-error", "deny",
		"modelo fora do ar ou lento: deny barra; rules usa as regras")
}

// #endregion

// #region cmd-guarita-new-classifier
// newClassifier monta o inspetor e o aquece antes de a Guarita abrir
// a escuta. Sem -classifier, não há inspetor.
func newClassifier(
	ctx context.Context, c classifierConfig, logger *slog.Logger,
) (app.ResultInspector, error) {
	if c.endpoint == "" {
		return nil, nil
	}
	options, err := classifierOptions(c)
	if err != nil {
		return nil, err
	}
	client, err := ollama.New(c.endpoint, c.model)
	if err != nil {
		return nil, err
	}
	inspector := classifier.New(client, c.threshold, options...)
	if err := warmUp(ctx, client, inspector, logger); err != nil {
		return nil, err
	}
	return inspector, nil
}

// #endregion

// #region cmd-guarita-warm-up
// warmUp confere que o modelo está instalado e faz a primeira
// pergunta, com um minuto de prazo: carregar o modelo na memória leva
// mais que o prazo de um resultado, e a primeira chamada do agente
// seria barrada.
func warmUp(
	ctx context.Context, client *ollama.Client,
	inspector *classifier.Inspector, logger *slog.Logger,
) error {
	warm, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	digest, err := client.Digest(warm)
	if err != nil {
		return err
	}
	start := time.Now()
	if _, err := inspector.Score(warm, "aquecimento"); err != nil {
		return err
	}
	logger.Info("classificador pronto", "model", client.Model(),
		"digest", digest, "warmup", time.Since(start).String())
	return nil
}

// #endregion

// #region cmd-guarita-classifier-options
// classifierOptions traduz as opções. O plano B é escolha explícita
// do operador: por padrão, sem o modelo, o resultado é barrado.
func classifierOptions(
	c classifierConfig,
) ([]classifier.Option, error) {
	if c.threshold <= 0 || c.threshold > 1 || c.timeout <= 0 {
		return nil, fmt.Errorf("%w: limiar %v, prazo %v", errClassifier,
			c.threshold, c.timeout)
	}
	options := []classifier.Option{classifier.WithTimeout(c.timeout)}
	switch c.onError {
	case "deny":
		return options, nil
	case "rules":
		return append(options, classifier.WithFallback(
			injection.New(injection.DefaultActions()))), nil
	}
	return nil, fmt.Errorf("%w: -classifier-on-error %q", errClassifier,
		c.onError)
}

// #endregion
