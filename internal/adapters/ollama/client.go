// Package ollama fala com a API HTTP de um Ollama local, só no que o
// classificador usa: POST /api/chat, com as probabilidades do primeiro
// token, e GET /api/tags, para conferir o modelo.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Erros do cliente. Um erro de rede ou de prazo vem embrulhado como o
// net/http o devolve, e errors.Is(err, context.DeadlineExceeded) vale.
var (
	ErrEndpoint      = errors.New("ollama: invalid endpoint")
	ErrStatus        = errors.New("ollama: unexpected status")
	ErrNoLogprobs    = errors.New("ollama: response without logprobs")
	ErrNoAnswer      = errors.New("ollama: neither YES nor NO in reply")
	ErrModelNotFound = errors.New("ollama: model not found")
)

// maxResponse limita o corpo lido: a resposta de um token cabe em
// poucos kilobytes, e um servidor errado não enche a memória.
const maxResponse = 1 << 20

// #region ollama-client
// Client é o cliente de um modelo num Ollama. O http.Client tem um
// prazo total de 30 segundos, que é a última barreira: o prazo de cada
// pergunta vem do contexto de quem chama, e é bem menor.
type Client struct {
	base      *url.URL
	model     string
	keepAlive string
	http      *http.Client
}

// Option ajusta o cliente.
type Option func(*Client)

// New valida o endereço e cria o cliente do modelo dado.
func New(endpoint, model string, options ...Option) (*Client, error) {
	base, err := url.Parse(endpoint)
	if err != nil || base.Host == "" ||
		(base.Scheme != "http" && base.Scheme != "https") {
		return nil, fmt.Errorf("%w: %q", ErrEndpoint, endpoint)
	}
	c := &Client{
		base: base, model: model, keepAlive: "30m",
		http: &http.Client{Timeout: 30 * time.Second},
	}
	for _, option := range options {
		option(c)
	}
	return c, nil
}

// #endregion

// WithHTTPClient troca o http.Client, com outro prazo ou transporte.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) { c.http = client }
}

// WithKeepAlive diz ao Ollama por quanto tempo manter o modelo na
// memória depois da última pergunta, no formato do Ollama ("30m",
// "-1" para sempre). Carregar o modelo de novo leva segundos.
func WithKeepAlive(keepAlive string) Option {
	return func(c *Client) { c.keepAlive = keepAlive }
}

// Model devolve o nome do modelo, como o Ollama o conhece.
func (c *Client) Model() string { return c.model }

// #region ollama-request
// chatRequest é o corpo de POST /api/chat. Temperatura zero e semente
// fixa tornam a resposta repetível; um único token basta para YES ou
// NO, e as dez alternativas mais prováveis dão a probabilidade.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream"`
	Logprobs    bool      `json:"logprobs"`
	TopLogprobs int       `json:"top_logprobs"`
	KeepAlive   string    `json:"keep_alive,omitempty"`
	Options     struct {
		Temperature float64 `json:"temperature"`
		Seed        int     `json:"seed"`
		NumPredict  int     `json:"num_predict"`
	} `json:"options"`
}

// #endregion

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

type chatResponse struct {
	Logprobs []struct {
		TopLogprobs []tokenLogprob `json:"top_logprobs"`
	} `json:"logprobs"`
}

// #region ollama-yes
// YesProbability faz uma pergunta de sim ou não e devolve a fração da
// probabilidade do primeiro token que ficou com YES, entre YES e NO.
func (c *Client) YesProbability(
	ctx context.Context, system, user string,
) (float64, error) {
	request := chatRequest{
		Model: c.model, Logprobs: true, TopLogprobs: 10,
		KeepAlive: c.keepAlive,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	request.Options.Seed, request.Options.NumPredict = 15, 1
	var reply chatResponse
	if err := c.post(ctx, "/api/chat", request, &reply); err != nil {
		return 0, err
	}
	if len(reply.Logprobs) == 0 {
		return 0, ErrNoLogprobs
	}
	return yesShare(reply.Logprobs[0].TopLogprobs)
}

// #endregion

// #region ollama-yes-share
// yesShare soma as variantes de cada resposta (" YES", "Yes") e
// normaliza entre as duas: o resto da massa é de tokens que não são
// resposta. Sem nenhuma das duas entre as dez, não há resposta.
func yesShare(top []tokenLogprob) (float64, error) {
	var yes, no float64
	for _, t := range top {
		switch strings.ToUpper(strings.TrimSpace(t.Token)) {
		case "YES":
			yes += math.Exp(t.Logprob)
		case "NO":
			no += math.Exp(t.Logprob)
		}
	}
	if yes+no == 0 {
		return 0, ErrNoAnswer
	}
	return yes / (yes + no), nil
}

// #endregion

// #region ollama-digest
// Digest confere que o modelo está instalado e devolve o resumo dele:
// o nome "qwen2.5:7b" pode apontar para outro arquivo amanhã, e o
// resumo diz qual modelo produziu cada medida.
func (c *Client) Digest(ctx context.Context) (string, error) {
	var tags struct {
		Models []struct{ Name, Digest string } `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil,
		&tags); err != nil {
		return "", err
	}
	for _, m := range tags.Models {
		if m.Name == c.model {
			return m.Digest, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrModelNotFound, c.model)
}

// #endregion

func (c *Client) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, body, out)
}

// #region ollama-do
// do faz o pedido com o contexto de quem chama: se o prazo dele vencer
// antes, a conexão é desfeita e o erro diz por quê. O corpo lido tem
// limite, e um status fora de 200 é erro, sem tentar decodificar.
func (c *Client) do(
	ctx context.Context, method, path string, body []byte, out any,
) error {
	request, err := http.NewRequestWithContext(ctx, method,
		c.base.JoinPath(path).String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrStatus, response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxResponse)
	if err := json.NewDecoder(limited).Decode(out); err != nil {
		return fmt.Errorf("ollama: %w", err)
	}
	return nil
}

// #endregion
