package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/aluiziodeveloper/guarita/internal/adapters/admin"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

var errApprovalsUsage = errors.New("uso: guarita approvals " +
	"[-admin URL] -secret arquivo list | approve ID | deny ID")

// #region cmd-guarita-approvals
// runApprovals é o subcomando do operador. Ele tem o próprio conjunto
// de opções e fala com a interface do operador, nunca com o endpoint
// do agente.
func runApprovals(
	ctx context.Context, args []string, stdout, stderr io.Writer,
) error {
	flags := flag.NewFlagSet("approvals", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("admin", "http://127.0.0.1:8081",
		"endereço da interface do operador")
	secretFile := flags.String("secret", "", "segredo do operador")
	if err := flags.Parse(args); err != nil || *secretFile == "" {
		return errors.Join(errApprovalsUsage, err)
	}
	operator, err := secret.ReadFile(*secretFile)
	if err != nil {
		return err
	}
	client := adminClient{base: *base, secret: operator}
	switch command := flags.Args(); {
	case len(command) == 1 && command[0] == "list":
		return client.list(ctx, stdout)
	case len(command) == 2 && command[0] == "approve",
		len(command) == 2 && command[0] == "deny":
		return client.decide(ctx, command[1], command[0])
	}
	return errApprovalsUsage
}

// #endregion

// adminClient fala com a interface do operador com o segredo dele.
type adminClient struct {
	base   string
	secret secret.Secret
}

func (c adminClient) do(
	ctx context.Context, method, path string,
) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+c.secret.Reveal())
	return http.DefaultClient.Do(r)
}

// #region cmd-guarita-approvals-list
// list escreve os pendentes em colunas: tabwriter alinha o texto
// separado por tabulações, como uma tabela de terminal.
func (c adminClient) list(ctx context.Context, stdout io.Writer) error {
	response, err := c.do(ctx, http.MethodGet, "/approvals")
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("list: %s", response.Status)
	}
	var items []admin.Item
	err = json.NewDecoder(response.Body).Decode(&items)
	if err != nil {
		return err
	}
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table,
		"ID\tAGENTE\tSERVIDOR\tFERRAMENTA\tARGUMENTOS")
	for _, item := range items {
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", item.ID,
			item.Agent, item.Server, item.Tool, item.Arguments)
	}
	return table.Flush()
}

// #endregion

// decide aprova ou nega; o identificador vai escapado no caminho.
func (c adminClient) decide(ctx context.Context, id, verb string) error {
	response, err := c.do(ctx, http.MethodPost,
		"/approvals/"+url.PathEscape(id)+"/"+verb)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("%s: %s %s", verb, response.Status,
			strings.TrimSpace(string(body)))
	}
	return nil
}
