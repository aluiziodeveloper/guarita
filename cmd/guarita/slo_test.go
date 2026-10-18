package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// slo é o orçamento de latência que a Guarita pode acrescentar a uma
// chamada, no p99, com os inspetores determinísticos ligados.
const slo = 5 * time.Millisecond

// calls é quantas chamadas cada cenário faz, em sequência.
const calls = 2000

// #region cmd-guarita-slo-test
// TestSLO mede a latência que a Guarita acrescenta: as mesmas chamadas
// direto ao servidor e por quatro configurações dela. Mede tempo, e
// por isso só roda com GUARITA_SLO=1, sem -race.
func TestSLO(t *testing.T) {
	if os.Getenv("GUARITA_SLO") == "" {
		t.Skip("GUARITA_SLO=1 para medir")
	}
	upstream := resultUpstream(t)
	direct := percentiles(t, upstream)
	t.Logf("%-28s p50 %-9v p95 %-9v p99 %v", "direto", direct[0],
		direct[1], direct[2])
	trail := filepath.Join(t.TempDir(), "auditoria.jsonl")
	for _, s := range sloScenarios(trail) {
		ctx, cancel := context.WithCancel(t.Context())
		endpoint, _ := startHTTP(t, ctx, append(s.args, "-listen",
			loopback, "-upstream", upstream)...)
		p := percentiles(t, endpoint)
		cancel()
		extra := p[2] - direct[2]
		t.Logf("%-28s p50 %-9v p95 %-9v p99 %-9v +p99 %v", s.name, p[0],
			p[1], p[2], extra)
		if s.budgeted && extra > slo {
			t.Errorf("%s: +%v no p99, acima do SLO de %v", s.name,
				extra, slo)
		}
	}
}

// #endregion

type sloScenario struct {
	name     string
	args     []string
	budgeted bool // se o SLO vale para ele
}

// #region cmd-guarita-slo-scenarios
// sloScenarios vão da política sozinha à linha determinística inteira
// e à trilha, que espera o disco a cada decisão.
func sloScenarios(trail string) []sloScenario {
	line := []string{
		"-allow", "read_file", "-dlp",
		"-inspect-results", "-mark-untrusted",
	}
	return []sloScenario{
		{"política", []string{"-allow", "read_file"}, true},
		{"política e inspetores", line, true},
		{"inspetores e métricas", append(slices.Clone(line),
			"-ops-listen", loopback), true},
		{"inspetores e trilha (fsync)", append(slices.Clone(line),
			"-audit", trail), false},
	}
}

// #endregion

// percentiles faz as chamadas e devolve o p50, o p95 e o p99.
func percentiles(t *testing.T, endpoint string) [3]time.Duration {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: 1,
	}}
	body := toolCall(1, "read_file")
	took := make([]time.Duration, 0, calls)
	for range calls {
		start := time.Now()
		res, err := client.Post(endpoint, "application/json",
			strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		took = append(took, time.Since(start))
	}
	slices.Sort(took)
	at := func(q float64) time.Duration {
		return took[int(q*float64(len(took)-1))].Round(time.Microsecond)
	}
	return [3]time.Duration{at(0.50), at(0.95), at(0.99)}
}

// resultUpstream responde a toda chamada com 1 KiB de texto, para os
// inspetores da volta terem o que examinar.
func resultUpstream(t *testing.T) string {
	t.Helper()
	text := strings.Repeat("Vendas estáveis no trimestre. ", 35)[:1024]
	reply := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{`+
		`"content":[{"type":"text","text":%q}]}}`, text)
	l, _ := net.Listen("tcp", loopback)
	server := &http.Server{Handler: http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, reply)
		}), ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })
	return "http://" + l.Addr().String() + "/mcp"
}
