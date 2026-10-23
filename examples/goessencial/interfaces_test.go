package goessencial

import (
	"fmt"
	"strings"
)

// #region interfaces-implicitas
// Uma interface é um conjunto de métodos, e um tipo a satisfaz só por
// ter os métodos, sem declarar nada. Quem consome declara a interface
// pequena de que precisa.
type inspector interface {
	Inspect(args string) (ok bool)
}

type dlp struct{}

func (dlp) Inspect(args string) bool { return !strings.Contains(args, "CPF") }

type allowAll struct{}

func (allowAll) Inspect(string) bool { return true }

func admit(args string, line ...inspector) bool {
	for _, i := range line {
		if !i.Inspect(args) {
			return false
		}
	}
	return true
}

func Example_interfaceImplicita() {
	fmt.Println(admit("relatorio.txt", allowAll{}, dlp{}))
	fmt.Println(admit("CPF 123", allowAll{}, dlp{}))
	// Output:
	// true
	// false
}

// #endregion

// #region interfaces-tipo
// any é a interface vazia. A asserção de tipo recupera o tipo
// concreto, e a forma com ok não entra em pânico quando ele é outro.
func describe(v any) string {
	switch x := v.(type) {
	case nil:
		return "nada"
	case string:
		return "texto " + x
	case fmt.Stringer:
		return "decisão " + x.String()
	default:
		return fmt.Sprintf("outro %T", x)
	}
}

func Example_switchDeTipo() {
	fmt.Println(describe(nil), "|", describe("ok"), "|", describe(Deny),
		"|", describe(3.5))
	var v any = "read_file"
	n, ok := v.(int)
	fmt.Println(n, ok)
	// Output:
	// nada | texto ok | decisão deny | outro float64
	// 0 false
}

// #endregion

// #region interfaces-nil
// Uma interface só é nil sem tipo e sem valor. Guardar nela um ponteiro
// nulo de um tipo concreto dá uma interface que não é nil, a armadilha
// do Capítulo 18.
type nullInspector struct{}

func (*nullInspector) Inspect(string) bool { return true }

func Example_interfaceComPonteiroNulo() {
	// O staticcheck acusa a comparação como sempre falsa (SA4023): é a
	// armadilha que o exemplo mostra.
	var p *nullInspector //nolint:staticcheck // SA4023
	var i inspector = p
	fmt.Println(p == nil, i == nil) //nolint:staticcheck // SA4023
	// Output: true false
}

// #endregion
