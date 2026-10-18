package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// startWithOps roda a Guarita e devolve a escuta do agente e a de
// operação, como ela as anuncia.
func startWithOps(t *testing.T, ctx context.Context, args ...string) (
	endpoint, ops string,
) {
	t.Helper()
	r, w := io.Pipe()
	go func() {
		_ = run(ctx, append([]string{"-insecure-no-auth"}, args...),
			io.NopCloser(strings.NewReader("")),
			nopWriteCloser{&bytes.Buffer{}}, w)
		_ = w.Close()
	}()
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		if a, ok := strings.CutPrefix(lines.Text(),
			"guarita: operação em "); ok {
			ops = strings.TrimSuffix(a, "/")
		}
		if a, ok := strings.CutPrefix(lines.Text(),
			"guarita: escutando em "); ok {
			go func() { _, _ = io.Copy(io.Discard, r) }()
			return a, ops
		}
	}
	t.Fatal("a Guarita não anunciou as escutas")
	return "", ""
}

// #region cmd-guarita-ops-test
// Uma chamada permitida e uma negada, e a escuta de operação: saúde,
// prontidão, as medidas das duas mensagens e os perfis, só com -pprof
// e só com o Host do loopback.
func TestRun_operacao(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	endpoint, ops := startWithOps(t, ctx, "-allow", "read_file",
		"-listen", loopback, "-ops-listen", loopback, "-pprof",
		"-upstream", echoUpstream(t))
	expect(t, endpoint, "", toolCall(1, "read_file"), 200, `"result"`)
	expect(t, endpoint, "", toolCall(2, "delete_file"), 200, `"error"`)
	for _, c := range opsCases {
		code, body := get(t, ops+c.path, c.host)
		ok := code == c.code && strings.Contains(body, c.want)
		t.Logf("%-14s %-9s %d %t", c.path, c.host, code, ok)
		if !ok {
			t.Errorf("%s: %d %.80q", c.path, code, body)
		}
	}
}

// #endregion

// opsCases são as rotas de operação e o que cada uma responde.
var opsCases = []struct {
	path, host string
	code       int
	want       string
}{
	{"/healthz", "", 200, "ok"},
	{"/readyz", "", 200, "ready"},
	{"/metrics", "", 200, `{outcome="denied"} 1`},
	{"/debug/pprof/", "", 200, "goroutine"},
	{"/debug/pprof/", "evil.test", 403, "host not allowed"},
}

// #region cmd-guarita-ops-pprof-test
// Sem -pprof não há perfis; com -pprof fora do loopback, a Guarita não
// sobe. O probe health é o que o Compose executa.
func TestOps_pprofEHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	_, ops := startWithOps(t, ctx, "-allow", "read_file",
		"-listen", "127.0.0.1:0", "-ops-listen", "127.0.0.1:0",
		"-upstream", echoUpstream(t))
	code, _ := get(t, ops+"/debug/pprof/", "")
	_, err := opsConfig{pprof: true}.opsMux("0.0.0.0:9090", nil, nil)
	var out bytes.Buffer
	probe := []string{"health", "-url", ops + "/healthz"}
	herr := run(t.Context(), probe, nil, nopWriteCloser{&out},
		io.Discard)
	t.Logf("sem -pprof: %d; fora do loopback: %v; health: %s %v", code,
		err, strings.TrimSpace(out.String()), herr)
	if code != 404 || !errors.Is(err, errPprofLoopback) || herr != nil {
		t.Error("os perfis ou o probe não se comportaram")
	}
}

// #endregion

// loopback é uma porta livre do loopback.
const loopback = "127.0.0.1:0"

func get(t *testing.T, url, host string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if host != "" {
		req.Host = host
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}
