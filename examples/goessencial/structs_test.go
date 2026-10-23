package goessencial

import "fmt"

// #region structs-metodos
// Uma struct agrupa campos; os métodos são declarados fora dela, com
// um receptor. O receptor por valor recebe uma cópia; o receptor por
// ponteiro altera o original.
type Quota struct {
	Limit, Used int
}

func (q Quota) Left() int { return q.Limit - q.Used }

func (q *Quota) Take() bool {
	if q.Used >= q.Limit {
		return false
	}
	q.Used++
	return true
}

func Example_receptores() {
	q := Quota{Limit: 2}
	fmt.Println(q.Take(), q.Take(), q.Take(), q.Left())
	copied := q // atribuir uma struct copia os campos
	copied.Used = 0
	fmt.Println(q.Used, copied.Used, q == Quota{Limit: 2, Used: 2})
	// Output:
	// true true false 0
	// 2 0 true
}

// #endregion

// #region structs-embutir
// Go não tem herança. Um campo embutido, sem nome, promove os campos e
// os métodos dele para a struct de fora: é composição, e Named continua
// tendo uma Quota, e não sendo uma.
type Named struct {
	Quota
	Tool string
}

func Example_embutimento() {
	n := Named{Quota: Quota{Limit: 3, Used: 1}, Tool: "read_file"}
	n.Take() // promovido: o mesmo que n.Quota.Take()
	fmt.Println(n.Tool, n.Left(), n.Used)
	// Output: read_file 1 2
}

// #endregion
