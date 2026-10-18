package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// #region cmd-guarita-version-test
// O binário compilado como a release o compila diz de onde veio: a
// versão gravada por -ldflags, o Go, o commit e as opções de build.
func TestVersion_origem(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "guarita")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags",
		"-s -w -X main.version=v0.20.0-lab", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, msg)
	}
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("guarita version:\n%s", out)
	for _, want := range []string{
		"guarita v0.20.0-lab", "go1.27", "-trimpath=true",
		"CGO_ENABLED=0", "vcs.revision=",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("falta %q", want)
		}
	}
}

// #endregion
