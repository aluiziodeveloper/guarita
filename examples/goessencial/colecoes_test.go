package goessencial

import (
	"fmt"
	"maps"
	"slices"
)

// #region colecoes-fatia
// Uma fatia é uma janela sobre um vetor: ponteiro, tamanho e
// capacidade. Duas fatias podem dividir o mesmo vetor, e escrever numa
// muda a outra; slices.Clone copia os elementos.
func Example_fatiaCompartilhada() {
	base := make([]string, 2, 4)
	base[0], base[1] = "read_file", "list_dir"
	alias := base[:2]
	alias[0] = "delete_file"
	own := slices.Clone(base)
	own[1] = "send_email"
	fmt.Println(base, own, len(base), cap(base))
	// Output: [delete_file list_dir] [delete_file send_email] 2 4
}

// #endregion

// #region colecoes-append
// append devolve a fatia nova, e o resultado precisa ser guardado.
// Enquanto há capacidade, ele escreve no mesmo vetor.
func Example_append() {
	var tools []string // a fatia nula funciona com len e append
	fmt.Println(tools == nil, len(tools))
	tools = append(tools, "read_file", "list_dir")
	fmt.Println(tools, len(tools))
	// Output:
	// true 0
	// [read_file list_dir] 2
}

// #endregion

// #region colecoes-mapa
// Ler uma chave ausente devolve o valor zero; a segunda forma da
// leitura diz se a chave existe. A ordem de iteração de um mapa não é
// garantida, e por isso as chaves são ordenadas antes de imprimir.
func Example_mapa() {
	quota := map[string]int{"read_file": 20, "send_email": 3}
	fmt.Println(quota["delete_file"])
	if _, ok := quota["delete_file"]; !ok {
		fmt.Println("delete_file sem cota")
	}
	delete(quota, "send_email")
	quota["list_dir"] = 10
	for _, name := range slices.Sorted(maps.Keys(quota)) {
		fmt.Println(name, quota[name])
	}
	// Output:
	// 0
	// delete_file sem cota
	// list_dir 10
	// read_file 20
}

// #endregion
