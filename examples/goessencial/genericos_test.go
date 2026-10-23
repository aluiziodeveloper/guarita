package goessencial

import (
	"cmp"
	"fmt"
	"strings"
)

// #region genericos-funcoes
// Os parâmetros de tipo vêm entre colchetes, com uma restrição: any
// aceita qualquer tipo; cmp.Ordered, os que têm < e >. O compilador
// infere os tipos pelos argumentos.
func Map[T, U any](in []T, f func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func Largest[T cmp.Ordered](first T, rest ...T) T {
	m := first
	for _, v := range rest {
		m = max(m, v) // max e min são embutidas desde o Go 1.21
	}
	return m
}

func Example_funcoesGenericas() {
	fmt.Println(Map([]string{"dlp", "quota"}, strings.ToUpper))
	fmt.Println(Largest(3, 9, 4), Largest("b", "a"))
	// Output:
	// [DLP QUOTA]
	// 9 b
}

// #endregion

// #region genericos-tipo
// Um tipo também pode ser genérico. comparable restringe aos tipos que
// aceitam ==, os únicos que podem ser chave de mapa.
type Set[T comparable] struct {
	items map[T]struct{}
}

func NewSet[T comparable](values ...T) *Set[T] {
	s := &Set[T]{items: make(map[T]struct{})}
	for _, v := range values {
		s.items[v] = struct{}{}
	}
	return s
}

func (s *Set[T]) Has(v T) bool {
	_, ok := s.items[v]
	return ok
}

func Example_tipoGenerico() {
	allowed := NewSet("read_file", "list_dir")
	fmt.Println(allowed.Has("read_file"), allowed.Has("delete_file"))
	// Output: true false
}

// #endregion
