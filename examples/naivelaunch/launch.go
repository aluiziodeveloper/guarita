// Package naivelaunch mostra a aprovação ingênua de um servidor stdio,
// do Capítulo 20: pelo nome, achado no PATH, e pela versão que ele
// mesmo declara.
package naivelaunch

import (
	"os/exec"
	"strings"
)

// #region examples-naivelaunch
// Approved pergunta a versão ao servidor e a compara com a aprovada. O
// nome é procurado no PATH, como faria o shell, e a resposta é a que o
// próprio programa escolher dar.
func Approved(name, want string) (bool, error) {
	out, err := exec.Command(name, "--version").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == want, nil
}

// #endregion
