// Package naiveaudit mostra o registro ingênuo do Capítulo 19: cada
// chamada vira uma linha de JSON num arquivo, com os argumentos.
package naiveaudit

import (
	"io"
	"log/slog"
)

// #region examples-naiveaudit
// Logger registra cada chamada com slog, em JSON. Duas coisas estão
// erradas, e nenhuma aparece no uso: os argumentos vão inteiros para
// o registro, com o que houver neles, e qualquer um que escreva no
// arquivo muda uma linha sem deixar rastro.
func Logger(w io.Writer) func(agent, tool, decision, args string) {
	logger := slog.New(slog.NewJSONHandler(w, nil))
	return func(agent, tool, decision, args string) {
		logger.Info("chamada", "agent", agent, "tool", tool,
			"decision", decision, "args", args)
	}
}

// #endregion
