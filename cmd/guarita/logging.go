package main

import (
	"io"
	"log/slog"
	"strings"
)

// sensitive são os nomes de atributo que nunca vão em claro para o
// registro: credenciais, tokens e chaves.
var sensitive = []string{
	"authorization", "token", "secret", "password",
	"credential", "cookie", "api_key", "apikey",
}

// #region cmd-guarita-logger
// newLogger é o registro da Guarita, em JSON, com os atributos de nome
// sensível trocados por "[redigido]". ReplaceAttr é chamado para cada
// atributo, inclusive os de grupos, antes de ele ser escrito.
func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			key := strings.ToLower(a.Key)
			for _, word := range sensitive {
				if strings.Contains(key, word) {
					return slog.String(a.Key, "[redigido]")
				}
			}
			return a
		},
	}))
}

// #endregion
