package supplychain

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// files compila o servidor de arquivos em dir/sub, a versão aprovada
// ou, com update, a atualização.
func files(t *testing.T, dir, sub string, update bool) string {
	t.Helper()
	var tags []string
	if update {
		tags = append(tags, "labupdate")
	}
	return build(t, filepath.Join(dir, sub), "lab-files-mcp", "lab-files",
		tags...)
}

// #region attacks-supplychain-same
// A versão aprovada e a atualização, lado a lado: o mesmo nome, a mesma
// versão e as mesmas definições, que o catálogo do Capítulo 9 fixaria
// com o mesmo digest. Só o digest do executável muda.
func TestAtualizacao_mesmasDefinicoes(t *testing.T) {
	dir := t.TempDir()
	v1, v2 := files(t, dir, "v1", false), files(t, dir, "v2", true)
	var seen [][]string
	for _, bin := range []string{v1, v2} {
		var stderr bytes.Buffer
		s, err := connect(t, &stderr, bin, "-root", dir)
		if err != nil {
			t.Fatal(err)
		}
		info := s.InitializeResult().ServerInfo
		f := fingerprints(t, s)
		_ = s.Close()
		t.Logf("%s: %s %s; definição %.23s…; executável %.23s…; "+
			"stderr %q", filepath.Base(filepath.Dir(bin)), info.Name,
			info.Version, f[0], digest(t, bin),
			strings.TrimSpace(stderr.String()))
		seen = append(seen, f)
	}
	same := slices.Equal(seen[0], seen[1])
	if !same || digest(t, v1) == digest(t, v2) {
		t.Error("a atualização deveria ter as mesmas definições")
	}
}

// #endregion

// #region attacks-supplychain-guarita
// A Guarita com o digest da v1: a v1 passa; a atualização no lugar do
// arquivo aprovado e a mesma atualização antes no PATH são recusadas
// antes de o servidor rodar, e o marcador não aparece.
func TestAtualizacao_digestAprovado(t *testing.T) {
	dir := t.TempDir()
	guarita := build(t, dir, "guarita", "guarita")
	approved := filepath.Dir(files(t, dir, "aprovado", false))
	pin := string(digest(t, filepath.Join(approved, "lab-files")))
	replaced := filepath.Dir(files(t, dir, "trocado", true))
	shadow := filepath.Dir(files(t, dir, "antes-no-path", true))
	for _, c := range []struct{ name, path string }{
		{"versão aprovada", approved},
		{"arquivo trocado", replaced},
		{"antes no PATH", shadow + ":" + approved},
	} {
		t.Setenv("PATH", c.path+":"+os.Getenv("PATH"))
		var stderr bytes.Buffer
		s, err := connect(t, &stderr, guarita, "-allow", "read_file",
			"-server-digest", pin, "--", "lab-files", "-root", dir)
		closeIf(s, err)
		t.Logf("%-16s sessão: %-5v %s", c.name, err == nil,
			lastLine(stderr.String()))
		if (c.name == "versão aprovada") != (err == nil) ||
			strings.Contains(stderr.String(), "[LAB]") {
			t.Errorf("%s: %v\n%s", c.name, err, stderr.String())
		}
	}
}

// #endregion

// closeIf fecha a sessão, se ela abriu.
func closeIf(s interface{ Close() error }, err error) {
	if err == nil {
		_ = s.Close()
	}
}

// lastLine é a última linha não vazia, encurtada.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	line := lines[len(lines)-1]
	if len(line) > 90 {
		line = line[:90] + "…"
	}
	return line
}
