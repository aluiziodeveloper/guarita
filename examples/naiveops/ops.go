// Package naiveops mostra a observabilidade ingênua do Capítulo 21: o
// pprof importado só pelo efeito e o serviço atendido pelo
// DefaultServeMux, o que põe /debug/pprof na mesma porta do agente.
package naiveops

import (
	"io"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: o exemplo é o defeito
)

// #region examples-naiveops
// Handler é o serviço do exemplo, registrado no multiplexador padrão.
// O import de net/http/pprof, ao ser carregado, registrou ali também
// /debug/pprof/: quem alcança o serviço alcança os perfis.
func Handler() http.Handler {
	http.HandleFunc("/mcp", serve)
	return http.DefaultServeMux
}

func serve(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{}}`)
}

// #endregion
