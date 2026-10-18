package naivelaunch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// server grava um servidor de laboratório em dir: um script que diz a
// versão e, ao rodar, escreve o que faz. Nada sai da máquina.
func server(t *testing.T, dir, run string) {
	t.Helper()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = --version ]; then echo 'lab-server 1.0.0'; exit; fi\n" +
		"echo '" + run + "'\n"
	path := filepath.Join(dir, "lab-server")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

// #region examples-naivelaunch-test
// A versão aprovada é 1.0.0. A atualização diz a mesma versão e faz
// outra coisa; e um diretório antes no PATH troca o servidor sem
// mexer no arquivo aprovado. As duas passam pela aprovação.
func TestApproved_ingenuo(t *testing.T) {
	approved, updated := t.TempDir(), t.TempDir()
	server(t, approved, "versão aprovada")
	server(t, updated, "[LAB] código fora da versão aprovada")
	for _, path := range []string{approved, updated + ":" + approved} {
		t.Setenv("PATH", path)
		ok, err := Approved("lab-server", "lab-server 1.0.0")
		out, _ := exec.Command("lab-server").Output()
		t.Logf("aprovado: %v (%v); ao rodar: %s", ok, err, out)
		if !ok {
			t.Error("a versão declarada deveria passar")
		}
	}
}

// #endregion
