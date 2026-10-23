package goessencial

import (
	"fmt"
	"strconv"
)

// #region fluxo-if-switch
// O if aceita uma declaração antes da condição, com escopo no próprio
// if. O switch não cai para o caso seguinte, e um caso aceita vários
// valores.
func Example_ifESwitch() {
	for _, text := range []string{"20", "x"} {
		if n, err := strconv.Atoi(text); err != nil {
			fmt.Println("inválido:", text)
		} else {
			fmt.Println("cota:", n)
		}
	}
	for _, method := range []string{"tools/call", "ping", "resources/read"} {
		switch method {
		case "tools/call":
			fmt.Println(method, "passa pela política")
		case "ping", "initialize":
			fmt.Println(method, "segue direto")
		default:
			fmt.Println(method, "não é decidido")
		}
	}
	// Output:
	// cota: 20
	// inválido: x
	// tools/call passa pela política
	// ping segue direto
	// resources/read não é decidido
}

// #endregion

// #region fluxo-for
// for é o único laço. Ele conta, percorre fatias, mapas e canais e,
// desde o Go 1.22, também um inteiro: range 3 vai de 0 a 2.
func Example_for() {
	for i := range 3 {
		fmt.Print(i, " ")
	}
	n := 1
	for n < 100 { // o "while" de outras linguagens
		n *= 10
	}
	fmt.Println(n)
	// Output: 0 1 2 100
}

// #endregion

// #region fluxo-defer
// defer agenda a chamada para o fim da função, na ordem inversa, mesmo
// se ela voltar por um erro ou por um pânico.
func Example_defer() {
	func() {
		for i := range 3 {
			defer fmt.Print("fecha ", i, "; ")
		}
		fmt.Print("trabalha; ")
	}()
	fmt.Println()
	// Output: trabalha; fecha 2; fecha 1; fecha 0;
}

// #endregion
