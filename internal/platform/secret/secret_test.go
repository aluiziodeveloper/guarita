package secret

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #region secret-test
// Nenhuma forma comum de imprimir o segredo mostra o valor.
func TestSecret_naoSeDeixaImprimir(t *testing.T) {
	s := New("cred-da-guarita")
	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("config",
		"credential", s)
	encoded, _ := json.Marshal(struct{ C Secret }{s})
	for name, out := range map[string]string{
		"%v": fmt.Sprintf("%v", s), "%+v": fmt.Sprintf("%+v", s),
		"%#v": fmt.Sprintf("%#v", s), "%q": fmt.Sprintf("%q", s),
		"slog": logs.String(), "json": string(encoded),
	} {
		if strings.Contains(out, s.Reveal()) {
			t.Errorf("%s vazou o segredo: %s", name, out)
		}
	}
}

// #endregion

func TestReadFile_recusaArquivoAberto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(path, []byte("  cred\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); !errors.Is(err, ErrPermissions) {
		t.Fatalf("0644: err = %v; want ErrPermissions", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := ReadFile(path)
	if err != nil || s.Reveal() != "cred" {
		t.Fatalf("0600: %q, %v", s.Reveal(), err)
	}
}

// #region secret-private-test
// A regra inteira, sem precisar de arquivos do root: os demais nunca
// leem; o grupo, só com o arquivo do root e o grupo do processo.
func TestPrivate(t *testing.T) {
	me := os.Getegid()
	user, root := fileOwner{true, 1000, me}, fileOwner{true, 0, me}
	for _, c := range []struct {
		name string
		mode os.FileMode
		o    fileOwner
		want bool
	}{
		{"só o dono", 0o600, user, true},
		{"legível por todos", 0o644, root, false},
		{"grupo, arquivo do usuário", 0o640, user, false},
		{
			"grupo, do root, outro grupo", 0o440,
			fileOwner{true, 0, me + 1},
			false,
		},
		{"grupo, do root, meu grupo", 0o440, root, true},
		{"grupo, dono desconhecido", 0o440, fileOwner{}, false},
	} {
		got := private(c.mode, c.o)
		t.Logf("%-30s %v -> %v", c.name, c.mode, got)
		if got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// #endregion
