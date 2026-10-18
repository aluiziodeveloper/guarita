package dlp

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// SignalDestination é uma URL para um host fora da lista.
const SignalDestination = "destination"

// Option ajusta o inspetor.
type Option func(*Inspector)

// WithAllowedHosts liga a lista de destinos: toda URL nos argumentos
// precisa apontar para um host dela. "lab.test" vale só para ele;
// ".lab.test" vale para os subdomínios.
func WithAllowedHosts(hosts ...string) Option {
	return func(i *Inspector) {
		for _, h := range hosts {
			i.hosts = append(i.hosts, strings.ToLower(h))
		}
	}
}

// urlCandidate acha URLs dentro de um texto; "//host" no começo é a
// URL relativa ao esquema, que um cliente HTTP também segue.
var urlCandidate = regexp.MustCompile(
	`(?i)(?:\b(?:https?|wss?)://|^//)[^\s"'<>]+`)

// #region dlp-destination
// destination acha as URLs do texto cujo host não está na lista.
// url.Parse faz o que uma expressão regular erraria: em
// "https://lab.test@evil.test/", o host é evil.test.
func (i *Inspector) destination(field, text string) []domain.Finding {
	if i.hosts == nil {
		return nil
	}
	var findings []domain.Finding
	for _, candidate := range urlCandidate.FindAllString(
		strings.TrimSpace(text), -1) {
		u, err := url.Parse(candidate)
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		if !i.allowed(host) {
			findings = append(findings, domain.Finding{
				Field: field, Signal: SignalDestination, Detail: host,
			})
		}
	}
	return findings
}

// #endregion

// allowed confere o host contra a lista, exato ou por sufixo.
func (i *Inspector) allowed(host string) bool {
	for _, entry := range i.hosts {
		if host == entry || (strings.HasPrefix(entry, ".") &&
			(strings.HasSuffix(host, entry) || host == entry[1:])) {
			return true
		}
	}
	return false
}
