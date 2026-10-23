package goessencial

import (
	"errors"
	"fmt"
)

// #region erros-envolver
// Um erro é um valor que implementa Error() string. %w envolve um erro
// em outro sem perder o original, e errors.Is o procura na cadeia; a
// mensagem é para pessoas, e nunca é comparada como texto.
var ErrDenied = errors.New("call denied")

func decide(tool string) error {
	if tool == "delete_database" {
		return fmt.Errorf("policy: %s: %w", tool, ErrDenied)
	}
	return nil
}

func Example_errorsIs() {
	err := decide("delete_database")
	fmt.Println(err)
	fmt.Println(errors.Is(err, ErrDenied), decide("read_file") == nil)
	// Output:
	// policy: delete_database: call denied
	// true true
}

// #endregion

// #region erros-tipo
// Um tipo de erro carrega dados; errors.As acha na cadeia o primeiro
// erro desse tipo e o copia para a variável.
type LimitError struct {
	Tool  string
	Limit int
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s: limit of %d reached", e.Tool, e.Limit)
}

func Example_errorsAs() {
	err := fmt.Errorf("quota: %w", &LimitError{Tool: "web_search", Limit: 20})
	var limit *LimitError
	if errors.As(err, &limit) {
		fmt.Println(limit.Tool, limit.Limit)
	}
	joined := errors.Join(ErrDenied, err)
	fmt.Println(errors.Is(joined, ErrDenied), errors.As(joined, &limit))
	// Output:
	// web_search 20
	// true true
}

// #endregion
