// Package denylist mostra a defesa ingênua do Capítulo 11: uma lista
// global de ferramentas permitidas, igual para todo agente, com uma
// lista de negação por padrão de nome por cima.
package denylist

import "path"

// #region examples-denylist
// Policy permite o que está em Allow, para qualquer agente, a menos que
// o nome case com um dos padrões de Deny.
type Policy struct {
	Allow []string // vale para todos os agentes
	Deny  []string // padrões de path.Match, como "delete_*"
}

// Allowed não sabe quem pede: o agente nem é parâmetro.
func (p Policy) Allowed(tool string) bool {
	for _, pattern := range p.Deny {
		if matched, _ := path.Match(pattern, tool); matched {
			return false
		}
	}
	for _, allowed := range p.Allow {
		if allowed == tool {
			return true
		}
	}
	return false
}

// #endregion
