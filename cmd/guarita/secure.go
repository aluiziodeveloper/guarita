package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"net"
	"net/http"
)

var (
	errNoAuth = errors.New("-agent-key is required; " +
		"-insecure-no-auth accepts agents without a token, on loopback")
	errTLSRequired = errors.New(
		"-tls-cert and -tls-key are required when -host is set")
)

// tlsConfig é o certificado da escuta do agente.
type tlsConfig struct {
	cert, key string
}

// #region cmd-guarita-secure-flags
// secureFlags declara as opções dos padrões seguros. Aceitar o agente
// sem token deixou de ser o padrão: é uma escolha escrita na linha de
// comando, e só vale no loopback.
func secureFlags(flags *flag.FlagSet, c *config) {
	flags.BoolVar(&c.auth.insecure, "insecure-no-auth", false,
		"aceita o agente sem token, só em loopback (laboratório)")
	flags.StringVar(&c.tls.cert, "tls-cert", "",
		"certificado PEM da escuta do agente")
	flags.StringVar(&c.tls.key, "tls-key", "",
		"chave PEM do certificado")
}

// #endregion

// #region cmd-guarita-check-defaults
// checkDefaults recusa a partida que exporia o agente: fora do
// loopback, sem identidade ou sem TLS, o token e as chamadas
// atravessariam a rede em claro.
func (c config) checkDefaults() error {
	if len(c.hosts) == 0 {
		return nil
	}
	if c.auth.keyFile == "" {
		return errAuthRequired
	}
	if c.tls.cert == "" || c.tls.key == "" {
		return errTLSRequired
	}
	return nil
}

// #endregion

// scheme é o esquema que o agente usa para falar com a Guarita.
func (c config) scheme() string {
	if c.tls.cert != "" {
		return "https"
	}
	return "http"
}

// #region cmd-guarita-serve-tls
// serve atende em claro ou, com certificado, por TLS 1.3 ou mais novo.
func (c config) serve(server *http.Server, l net.Listener) error {
	if c.tls.cert == "" {
		return server.Serve(l)
	}
	server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	return server.ServeTLS(l, c.tls.cert, c.tls.key)
}

// #endregion
