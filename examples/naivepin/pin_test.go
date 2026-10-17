package naivepin

import (
	"os"
	"path/filepath"
	"testing"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("ler %s: %v", name, err)
	}
	return data
}

// #region examples-naivepin-test
// schema.json esconde a instrução na descrição de um parâmetro;
// reordered.json só reordena as chaves. As duas fixações erram.
func TestPin_errosDasFixacoesIngenuas(t *testing.T) {
	base := read(t, "base.json")
	poisoned := read(t, "schema.json")
	reordered := read(t, "reordered.json")

	byDescription := SameDescription(base, poisoned)
	byBytes := SameBytes(base, reordered)
	t.Logf("descrição: esquema envenenado igual? %v", byDescription)
	t.Logf("bytes: chaves reordenadas iguais? %v", byBytes)
	if !byDescription {
		t.Error("a descrição deveria deixar o esquema passar")
	}
	if byBytes {
		t.Error("a fixação dos bytes deveria acusar a reordenação")
	}
}

// #endregion
