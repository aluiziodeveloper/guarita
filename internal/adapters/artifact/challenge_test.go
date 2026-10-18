package artifact

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Desafio do Capítulo 20: fechar a janela entre conferir e executar.
// Stage copia o executável para dir enquanto calcula o digest, e só
// devolve o caminho da cópia se ele for o aprovado: a cópia é só da
// Guarita (diretório 0o700, arquivo 0o500), e trocar o original depois
// não muda o que vai rodar. Com outro digest, nada fica em dir. O
// pacote não compila até existir Stage.
func TestStage_desafio(t *testing.T) {
	src, dir := t.TempDir(), filepath.Join(t.TempDir(), "guarita")
	approved := executable(t, src, "#!/bin/sh\necho aprovado\n")
	staged, err := Stage(filepath.Join(src, "lab-server"), approved, dir)
	if err != nil {
		t.Fatal(err)
	}
	executable(t, src, "#!/bin/sh\necho '[LAB] trocado depois'\n")
	out, _ := exec.Command(staged).Output()
	info, _ := os.Stat(staged)
	dinfo, _ := os.Stat(dir)
	t.Logf("cópia: %s; arquivo %v; diretório %v", out, info.Mode().Perm(),
		dinfo.Mode().Perm())
	if string(out) != "aprovado\n" || info.Mode().Perm() != 0o500 ||
		dinfo.Mode().Perm() != 0o700 {
		t.Error("a cópia não é a aprovada, ou não é só da Guarita")
	}
	other := filepath.Join(t.TempDir(), "guarita")
	_, err = Stage(filepath.Join(src, "lab-server"), approved, other)
	left, _ := os.ReadDir(other)
	t.Logf("original trocado: %v; arquivos deixados: %d", err, len(left))
	if !errors.Is(err, ErrMismatch) || len(left) != 0 {
		t.Errorf("Stage aceitou o trocado ou deixou a cópia: %v", err)
	}
}
