package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// executable grava um executável de laboratório e devolve o digest.
func executable(t *testing.T, dir, body string) domain.Digest {
	t.Helper()
	path := filepath.Join(dir, "lab-server")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	d, _ := domain.DigestOf(strings.NewReader(body))
	return d
}

// #region artifact-test
// O mesmo nome em dois diretórios: Resolve segue o PATH e um link
// simbólico até o arquivo real, e Check só aceita o conteúdo aprovado.
func TestResolveECheck(t *testing.T) {
	approvedDir, otherDir := t.TempDir(), t.TempDir()
	links := t.TempDir()
	approved := executable(t, approvedDir, "#!/bin/sh\necho aprovado\n")
	executable(t, otherDir, "#!/bin/sh\necho '[LAB] outro'\n")
	link := filepath.Join(links, "lab-server")
	_ = os.Symlink(filepath.Join(otherDir, "lab-server"), link)
	shadowed := links + ":" + approvedDir
	for _, path := range []string{approvedDir, shadowed} {
		t.Setenv("PATH", path)
		resolved, err := Resolve("lab-server")
		if err != nil {
			t.Fatal(err)
		}
		err = Check(resolved, approved)
		want := filepath.Join(approvedDir, "lab-server")
		isApproved := resolved == want
		t.Logf("arquivo aprovado: %-5v %v", isApproved, err)
		if isApproved != (err == nil) ||
			(err != nil && !errors.Is(err, ErrMismatch)) {
			t.Errorf("%s: %v", resolved, err)
		}
	}
}

// #endregion

// #region artifact-image-test
// A imagem precisa vir pelo digest aprovado; uma etiqueta não serve.
func TestCheckImage(t *testing.T) {
	d := domain.Digest("sha256:" + strings.Repeat("ab", 32))
	for _, c := range []struct {
		ref  string
		want error
	}{
		{"lab@" + string(d), nil},
		{"lab:1.0", ErrUnpinned},
		{"lab@sha256:" + strings.Repeat("cd", 32), ErrMismatch},
	} {
		err := CheckImage(c.ref, d)
		t.Logf("%.24s… -> %v", c.ref, err)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v; quer %v", c.ref, err, c.want)
		}
	}
}

// #endregion
