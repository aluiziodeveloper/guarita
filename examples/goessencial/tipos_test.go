package goessencial

import (
	"fmt"
	"unicode/utf8"
)

// #region tipos-zero
// Toda variável nasce com o valor zero do tipo; não existe variável
// sem valor, nem null para os tipos que não são ponteiros.
func Example_valorZero() {
	var n int
	var s string
	var ok bool
	var p *int
	fmt.Printf("%d %q %v %v\n", n, s, ok, p == nil)
	// Output: 0 "" false true
}

// #endregion

// #region tipos-enum
// Decision é um tipo nomeado sobre int. As constantes usam iota, que
// conta a partir de zero dentro do bloco const, e String dá o nome
// que o fmt imprime.
type Decision int

const (
	Allow Decision = iota
	Deny
	Approve
)

func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case Approve:
		return "approve"
	}
	return fmt.Sprintf("Decision(%d)", int(d))
}

func Example_enumeracao() {
	d := Deny // := declara e infere o tipo
	fmt.Println(d, Approve, Decision(7))
	// Output: deny approve Decision(7)
}

// #endregion

// #region tipos-conversao
// Não há conversão implícita entre tipos numéricos, nem entre um tipo
// nomeado e o tipo de base: toda conversão é escrita.
func Example_conversao() {
	var limit int32 = 10
	var used int64 = 4
	left := int64(limit) - used
	d := Decision(1)
	fmt.Println(left, int(d), d)
	// Output: 6 1 deny
}

// #endregion

// #region tipos-string
// Uma string é uma sequência imutável de bytes, em geral UTF-8. len
// conta bytes; o range percorre runas, os pontos de código.
func Example_stringBytesERunas() {
	s := "ação"
	fmt.Println(len(s), utf8.RuneCountInString(s))
	for i, r := range s {
		fmt.Printf("%d:%c ", i, r)
	}
	fmt.Println()
	// Output:
	// 6 4
	// 0:a 1:ç 3:ã 5:o
}

// #endregion
