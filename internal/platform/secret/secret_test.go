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
