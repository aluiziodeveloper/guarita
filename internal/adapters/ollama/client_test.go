package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fake é um Ollama falso: guarda o último pedido e responde o corpo
// dado.
func fake(t *testing.T, status int, body string) (*Client, *chatRequest) {
	t.Helper()
	var got chatRequest
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/chat" {
				_ = json.NewDecoder(r.Body).Decode(&got)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
	t.Cleanup(server.Close)
	client, err := New(server.URL, "lab:1b")
	if err != nil {
		t.Fatal(err)
	}
	return client, &got
}

// logprobs monta uma resposta com as alternativas do primeiro token.
func logprobs(pairs ...any) string {
	var top []string
	for i := 0; i < len(pairs); i += 2 {
		top = append(top, `{"token":"`+pairs[i].(string)+`","logprob":`+
			jsonNumber(math.Log(pairs[i+1].(float64)))+`}`)
	}
	return `{"message":{"content":"x"},"logprobs":[{"top_logprobs":[` +
		strings.Join(top, ",") + `]}]}`
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// #region ollama-test-request
// O pedido sai repetível e curto: temperatura zero, semente fixa, um
// token, as probabilidades ligadas e o texto na mensagem do usuário.
func TestYesProbability_pedido(t *testing.T) {
	body := logprobs("YES", 0.9, "NO", 0.1)
	client, got := fake(t, http.StatusOK, body)
	p, err := client.YesProbability(t.Context(), "sistema", "texto")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P(YES)=%.2f pedido=%+v", p, *got)
	if !got.Logprobs || got.TopLogprobs != 10 || got.Stream ||
		got.Options.NumPredict != 1 || got.Options.Temperature != 0 ||
		got.Messages[1].Content != "texto" || got.Model != "lab:1b" {
		t.Errorf("pedido inesperado: %+v", *got)
	}
}

// #endregion

// #region ollama-test-share
// A probabilidade soma as variantes de cada resposta e ignora o resto.
func TestYesProbability_normaliza(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want float64
	}{
		{"só YES e NO", logprobs("YES", 0.6, "NO", 0.2), 0.75},
		{"variantes", logprobs("NO", 0.5, " Yes", 0.2, "yes", 0.2,
			"Maybe", 0.1), 4.0 / 9},
		{"NO quase certo", logprobs("NO", 0.999, "YES", 0.001), 0.001},
	} {
		client, _ := fake(t, http.StatusOK, c.body)
		got, err := client.YesProbability(t.Context(), "s", "u")
		t.Logf("%-15s P(YES)=%.4f", c.name, got)
		if err != nil || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: %v, %v; quer %v", c.name, got, err, c.want)
		}
	}
}

// #endregion

// #region ollama-test-errors
// Status ruim, resposta sem probabilidades, resposta sem YES nem NO e
// modelo que não está instalado: cada um com o seu erro.
func TestCliente_erros(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"status 500", 500, `{"error":"x"}`, ErrStatus},
		{
			"sem logprobs", 200, `{"message":{"content":"YES"}}`,
			ErrNoLogprobs,
		},
		{"sem resposta", 200, logprobs("Talvez", 0.9), ErrNoAnswer},
	} {
		client, _ := fake(t, c.status, c.body)
		_, err := client.YesProbability(t.Context(), "s", "u")
		t.Logf("%-12s %v", c.name, err)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v; quer %v", c.name, err, c.want)
		}
	}
	client, _ := fake(t, 200, `{"models":[{"name":"outro:7b"}]}`)
	if _, err := client.Digest(t.Context()); !errors.Is(err,
		ErrModelNotFound) {
		t.Errorf("modelo ausente: %v", err)
	}
}

// #endregion

// #region ollama-test-deadline
// O prazo do contexto desfaz a conexão com um Ollama que não responde.
// O http.Client tem 30 segundos; o teste termina em 50 milissegundos.
func TestYesProbability_prazo(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	client, _ := New(server.URL, "lab:1b")
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.YesProbability(ctx, "s", "u")
	elapsed := time.Since(start).Round(10 * time.Millisecond)
	t.Logf("depois de %v: %v", elapsed, err)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("quer DeadlineExceeded, veio %v", err)
	}
}

// #endregion

func TestDigest(t *testing.T) {
	client, _ := fake(t, 200,
		`{"models":[{"name":"lab:1b","digest":"abc123"}]}`)
	got, err := client.Digest(t.Context())
	if err != nil || got != "abc123" {
		t.Errorf("Digest() = %q, %v", got, err)
	}
}

func TestNew_endereco(t *testing.T) {
	for _, endpoint := range []string{
		"", "127.0.0.1:11434",
		"ftp://127.0.0.1", "http://",
	} {
		if _, err := New(endpoint, "m"); !errors.Is(err, ErrEndpoint) {
			t.Errorf("New(%q) = %v", endpoint, err)
		}
	}
}

func TestDo_limitaOCorpo(t *testing.T) {
	big := `{"models":[{"name":"` + strings.Repeat("a", maxResponse) +
		`"}]}`
	client, _ := fake(t, 200, big)
	if _, err := client.Digest(t.Context()); err == nil {
		t.Error("um corpo maior que o limite foi aceito")
	}
}
