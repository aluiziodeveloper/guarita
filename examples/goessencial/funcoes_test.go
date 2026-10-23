package goessencial

import (
	"fmt"
	"strings"
)

// #region funcoes-retornos
// Uma função devolve vários valores, e o erro vem por último, como
// um valor comum.
func splitTool(name string) (server, tool string, err error) {
	server, tool, ok := strings.Cut(name, "__")
	if !ok {
		return "", "", fmt.Errorf("sem servidor: %q", name)
	}
	return server, tool, nil
}

func Example_variosRetornos() {
	server, tool, err := splitTool("files__read_file")
	fmt.Println(server, tool, err)
	_, _, err = splitTool("read_file") // _ descarta um valor
	fmt.Println(err)
	// Output:
	// files read_file <nil>
	// sem servidor: "read_file"
}

// #endregion

// #region funcoes-closure
// Funções são valores. Um fechamento guarda as variáveis do escopo em
// que foi criado, e cada chamada de counter tem a sua contagem.
func counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

func Example_fechamento() {
	a, b := counter(), counter()
	a()
	a()
	fmt.Println(a(), b())
	// Output: 3 1
}

// #endregion

// #region funcoes-variadica
// Um parâmetro variádico recebe zero ou mais valores como uma fatia; a
// fatia existente é passada com ... no fim.
func joinAll(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

func Example_variadica() {
	stages := []string{"dlp", "quota"}
	fmt.Println(joinAll(",", "policy", "audit"), joinAll(",", stages...))
	// Output: policy,audit dlp,quota
}

// #endregion
