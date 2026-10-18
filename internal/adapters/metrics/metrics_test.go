package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"
)

// #region metrics-test
// Um servidor que demora 20 ms atrás de um proxy reverso medido: o
// tempo do pedido inclui os 20 ms, o do servidor também, e o da
// Guarita, a diferença, fica abaixo de 5 ms.
func TestMetrics_latenciaDaGuarita(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, "{}")
		}))
	t.Cleanup(slow.Close)
	m := New()
	front := measured(t, m, slow.URL)
	for range 3 {
		get(t, front)
	}
	text := scrape(m)
	for _, line := range []string{
		`guarita_request_seconds_bucket{le="0.01"} 0`,
		`guarita_upstream_seconds_bucket{le="0.01"} 0`,
		`guarita_overhead_seconds_bucket{le="0.005"} 3`,
		`guarita_overhead_seconds_count 3`,
	} {
		if !strings.Contains(text, line+"\n") {
			t.Errorf("falta %q", line)
		}
	}
	t.Logf("trecho:\n%s", grep(text, "overhead_seconds_bucket", 0, 6))
}

// #endregion

// #region metrics-admitter-test
// O porteiro conta o que repassou e o que negou.
func TestMetrics_contaMensagens(t *testing.T) {
	m := New()
	gate := m.Admitter(admitFunc(func(_ context.Context, msg []byte) (
		bool, []byte,
	) {
		return string(msg) == "ok", nil
	}))
	for _, msg := range []string{"ok", "não", "ok"} {
		gate.Admit(t.Context(), []byte(msg))
	}
	text := scrape(m)
	t.Logf("trecho:\n%s", grep(text, "messages_total{", 0, 2))
	if !strings.Contains(text, `{outcome="forwarded"} 2`) ||
		!strings.Contains(text, `{outcome="denied"} 1`) {
		t.Error(text)
	}
}

// #endregion

// measured põe um proxy reverso medido na frente de target.
func measured(t *testing.T, m *Metrics, target string) string {
	t.Helper()
	u, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = m.Transport(nil)
	front := httptest.NewServer(m.Middleware(proxy))
	t.Cleanup(front.Close)
	return front.URL
}

func get(t *testing.T, url string) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec // servidor do teste
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
}

func scrape(m *Metrics) string {
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

// grep devolve as linhas com s, da posição from à to.
func grep(text, s string, from, to int) string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, s) {
			out = append(out, line)
		}
	}
	return strings.Join(out[from:min(to, len(out))], "\n")
}
