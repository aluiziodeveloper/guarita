package main

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"regexp"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/app"
)

// upstreamServer é um servidor MCP atrás da Guarita. O nome é o
// namespace dado pelo operador; vazio, é o servidor de -upstream, em
// /mcp, como até o Capítulo 9.
type upstreamServer struct {
	name       string
	url        string
	credential string // arquivo com a credencial só deste servidor
}

// validServerName limita o namespace a um trecho de caminho seguro.
var validServerName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

var errServerName = errors.New(
	"-server must be name=URL, name in [a-z0-9-], at most 32, unique")

// errCredential recusa a credencial que não tem dono certo.
var errCredential = errors.New("-server-credential must be name=file " +
	"of a -server; -upstream-credential needs -upstream")

// #region cmd-guarita-server-flag
// serverFlag lê -server nome=URL, que pode se repetir. O nome não vem
// do servidor (serverInfo não é único): é o operador quem o escolhe.
func serverFlag(servers *[]upstreamServer) func(string) error {
	return func(value string) error {
		name, target, ok := strings.Cut(value, "=")
		if !ok || !validServerName.MatchString(name) || target == "" {
			return errServerName
		}
		for _, existing := range *servers {
			if existing.name == name {
				return errServerName
			}
		}
		server := upstreamServer{name: name, url: target}
		*servers = append(*servers, server)
		return nil
	}
}

// #endregion

// resolveServers põe o servidor de -upstream, se houver, primeiro (ele
// fica em /mcp, como até o Capítulo 9), entrega as credenciais e
// confere os nomes de rota de -pipeline.
func (c *config) resolveServers(upstream string) error {
	if upstream != "" {
		c.servers = append([]upstreamServer{{url: upstream}},
			c.servers...)
	}
	if err := c.attachCredentials(); err != nil {
		return err
	}
	return c.checkRoutes()
}

// credentialFlag lê -server-credential nome=arquivo.
func credentialFlag(c *config) func(string) error {
	return func(value string) error {
		name, file, ok := strings.Cut(value, "=")
		if _, repeated := c.credentials[name]; !ok || file == "" ||
			repeated {
			return errCredential
		}
		if c.credentials == nil {
			c.credentials = map[string]string{}
		}
		c.credentials[name] = file
		return nil
	}
}

// #region cmd-guarita-attach-credentials
// attachCredentials dá a cada servidor só a sua credencial. Uma para
// todos entregaria a de um servidor a qualquer outro, inclusive a um
// hostil: o roubo de credencial do Capítulo 8, entre servidores.
func (c *config) attachCredentials() error {
	unused := maps.Clone(c.credentials)
	for i, server := range c.servers {
		if server.name == "" {
			c.servers[i].credential = c.auth.credentialFile
			continue
		}
		c.servers[i].credential = unused[server.name]
		delete(unused, server.name)
	}
	if len(unused) > 0 || (c.auth.credentialFile != "" &&
		(len(c.servers) == 0 || c.servers[0].name != "")) {
		return errors.Join(errUsage, errCredential)
	}
	return nil
}

// #endregion

// path é onde o agente alcança o servidor na Guarita.
func (s upstreamServer) path() string {
	if s.name == "" {
		return "/mcp"
	}
	return "/mcp/" + s.name
}

// catalogKey identifica o servidor no catálogo: o namespace, ou a URL
// para o servidor de -upstream, como no Capítulo 9.
func (s upstreamServer) catalogKey() string {
	if s.name == "" {
		return s.url
	}
	return s.name
}

// #region cmd-guarita-router
// routes atende /mcp/{server}: PathValue devolve o trecho do caminho
// que casou com o curinga, e um nome sem servidor recebe 404.
type routes map[string]http.Handler

func (rs routes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, ok := rs[r.PathValue("server")]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintln(w, `{"jsonrpc":"2.0","id":null,`+
			`"error":{"code":-32600,"message":"unknown server"}}`)
		return
	}
	handler.ServeHTTP(w, r)
}

// #endregion

// logRejection registra cada definição que não chegou ao agente.
func logRejection(logger *slog.Logger) func(app.Rejection) {
	return func(r app.Rejection) {
		signals := make([]string, 0, len(r.Findings))
		for _, f := range r.Findings {
			signals = append(signals, f.Field+":"+f.Signal+":"+f.Detail)
		}
		logger.Warn("definição recusada", "server", r.Server,
			"tool", r.Tool, "reason", r.Reason, "signals", signals)
	}
}
