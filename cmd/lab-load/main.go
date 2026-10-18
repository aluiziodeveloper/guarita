// Command lab-load é o gerador de carga do laboratório do Capítulo 21.
// Com -serve, ele é um servidor MCP de mentira, que responde a toda
// chamada com 1 KiB de texto; com -target, ele manda chamadas a uma
// Guarita, em paralelo, e diz quantas por segundo passaram. Ele só
// conversa com endereços do loopback, que o próprio laboratório abre.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var errNotLoopback = errors.New("lab-load só fala com o loopback")

func main() {
	serve := flag.String("serve", "", "escuta do servidor de mentira")
	target := flag.String("target", "", "URL da Guarita")
	calls := flag.Int("calls", 20000, "chamadas no total")
	workers := flag.Int("workers", 8, "chamadas em paralelo")
	flag.Parse()
	var err error
	switch {
	case *serve != "":
		err = serveFake(*serve)
	case *target != "":
		err = load(*target, *calls, *workers)
	default:
		err = errors.New("use -serve ou -target")
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// #region lab-load-load
// load reparte as chamadas entre os workers e mede a vazão total.
func load(target string, calls, workers int) error {
	if !strings.HasPrefix(target, "http://127.0.0.1:") {
		return errNotLoopback
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
		`"params":{"name":"read_file"}}`
	var next, failed atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for range workers {
		wg.Go(func() {
			for next.Add(1) <= int64(calls) {
				if post(target, body) != nil {
					failed.Add(1)
				}
			}
		})
	}
	wg.Wait()
	took := time.Since(start)
	fmt.Printf("%d chamadas em %v: %.0f por segundo; %d falhas\n",
		calls, took.Round(time.Millisecond),
		float64(calls)/took.Seconds(), failed.Load())
	return nil
}

// #endregion

// post manda uma chamada e lê a resposta inteira.
func post(target, body string) error {
	//nolint:gosec // G107: target foi conferido no loopback
	res, err := http.Post(target, "application/json",
		strings.NewReader(body))
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return res.Body.Close()
}

// serveFake responde a toda chamada com o mesmo resultado.
func serveFake(addr string) error {
	host, _, _ := net.SplitHostPort(addr)
	if host != "127.0.0.1" {
		return errNotLoopback
	}
	text := strings.Repeat("Vendas estáveis no trimestre. ", 35)[:1024]
	reply := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{`+
		`"content":[{"type":"text","text":%q}]}}`, text)
	server := &http.Server{
		Addr: addr, ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter,
			_ *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, reply)
		}),
	}
	return server.ListenAndServe()
}
