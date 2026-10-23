package goessencial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// #region concorrencia-canal
// go inicia uma goroutine. Um canal sem buffer entrega cada valor de
// mão em mão; quem fecha o canal é quem escreve, e o range termina
// quando ele é fechado.
func Example_canal() {
	results := make(chan string)
	go func() {
		defer close(results)
		for _, tool := range []string{"read_file", "list_dir"} {
			results <- "ok " + tool
		}
	}()
	for r := range results {
		fmt.Println(r)
	}
	// Output:
	// ok read_file
	// ok list_dir
}

// #endregion

// #region concorrencia-select
// select espera o primeiro de vários canais. Com o Done de um contexto,
// a espera tem prazo, e o erro do contexto diz por que ela acabou.
func Example_selectComPrazo() {
	ctx, cancel := context.WithTimeout(context.Background(),
		10*time.Millisecond)
	defer cancel()
	approval := make(chan bool) // ninguém responde
	select {
	case ok := <-approval:
		fmt.Println("aprovada:", ok)
	case <-ctx.Done():
		fmt.Println(ctx.Err(), errors.Is(ctx.Err(), context.DeadlineExceeded))
	}
	// Output: context deadline exceeded true
}

// #endregion

// #region concorrencia-sync
// WaitGroup.Go, do Go 1.25, inicia a goroutine e a conta; Wait espera
// todas. Um contador compartilhado precisa de trava ou de um tipo
// atômico; sem eles, go test -race acusa a corrida.
func Example_waitGroup() {
	var wg sync.WaitGroup
	var calls atomic.Int64
	var mu sync.Mutex
	seen := map[int]bool{}
	for i := range 100 {
		wg.Go(func() {
			calls.Add(1)
			mu.Lock()
			seen[i%10] = true
			mu.Unlock()
		})
	}
	wg.Wait()
	fmt.Println(calls.Load(), len(seen))
	// Output: 100 10
}

// #endregion

// #region concorrencia-errgroup
// errgroup junta goroutines que podem falhar: Wait devolve o primeiro
// erro, e o contexto do grupo é cancelado para avisar as outras.
func Example_errgroup() {
	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error { return errors.New("upstream: connection refused") })
	g.Go(func() error {
		<-ctx.Done() // a falha da outra cancela esta
		return ctx.Err()
	})
	fmt.Println(g.Wait())
	// Output: upstream: connection refused
}

// #endregion
