package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// memoryStore é o dublê do armazenamento, com a mesma semântica do
// SQLite: chave por servidor e ferramenta.
type memoryStore struct {
	pins map[[2]string]domain.Pin
	fail error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{pins: map[[2]string]domain.Pin{}}
}

func (m *memoryStore) Pin(
	_ context.Context, server, tool string,
) (domain.Pin, bool, error) {
	pin, ok := m.pins[[2]string{server, tool}]
	return pin, ok, m.fail
}

func (m *memoryStore) SavePin(_ context.Context, pin domain.Pin) error {
	m.pins[[2]string{pin.Server, pin.Tool}] = pin
	return m.fail
}

func (m *memoryStore) MarkChanged(
	_ context.Context, server, tool string,
	seen domain.Fingerprint, definition []byte,
) error {
	pin := m.pins[[2]string{server, tool}]
	pin.Changed, pin.SeenFingerprint = true, seen
	pin.SeenDefinition = definition
	m.pins[[2]string{server, tool}] = pin
	return m.fail
}

func (m *memoryStore) Owner(
	_ context.Context, server, tool string,
) (string, bool, error) {
	for key := range m.pins {
		if key[0] != server && domain.Skeleton(key[1]) ==
			domain.Skeleton(tool) {
			return key[0], true, m.fail
		}
	}
	return "", false, m.fail
}

func tool(name, description string) domain.ToolDefinition {
	return domain.ToolDefinition{Name: name, Raw: []byte(`{"name":"` +
		name + `","description":"` + description + `"}`)}
}

func names(definitions []domain.ToolDefinition) []string {
	var out []string
	for _, d := range definitions {
		out = append(out, d.Name)
	}
	return out
}

// #region app-catalog-test
// Primeira lista: tudo fixado. Segunda: read_file mudou e some.
// Terceira: voltou ao original, mas segue barrada até a revisão.
func TestPinCatalog_barraADefinicaoQueMudou(t *testing.T) {
	store := newMemoryStore()
	catalog := NewPinCatalog(store, "http://files.lab.test/mcp")
	ctx := context.Background()
	original := tool("read_file", "Lê um arquivo.")
	poisoned := tool("read_file", "Lê um arquivo. [LAB] alterada.")
	list := tool("list_dir", "Lista um diretório.")

	for i, round := range [][]domain.ToolDefinition{
		{original, list}, {poisoned, list}, {original, list},
	} {
		kept, err := catalog.Review(ctx, round)
		t.Logf("lista %d: chega ao agente %v", i+1, names(kept))
		if err != nil || (i > 0) == (len(kept) == 2) {
			t.Fatalf("lista %d: %v, %v", i+1, names(kept), err)
		}
	}
	if err := catalog.Check(ctx, "read_file"); !errors.Is(err,
		ErrDefinitionChanged) {
		t.Fatalf("Check(read_file) = %v", err)
	}
	if err := catalog.Check(ctx, "list_dir"); err != nil {
		t.Fatalf("Check(list_dir) = %v", err)
	}
}

// #endregion

func TestPinCatalog_falhaFechado(t *testing.T) {
	store := newMemoryStore()
	catalog := NewPinCatalog(store, "srv")
	ctx := context.Background()
	if err := catalog.Check(ctx, "never_listed"); !errors.Is(err,
		ErrNotInCatalog) {
		t.Errorf("fora do catálogo: %v", err)
	}
	invalid := domain.ToolDefinition{Name: "x", Raw: []byte(`[1]`)}
	if kept, _ := catalog.Review(ctx, []domain.ToolDefinition{invalid}); len(kept) != 0 {
		t.Errorf("definição inválida chegou ao agente")
	}
	store.fail = errors.New("disco cheio")
	if kept, err := catalog.Review(ctx, []domain.ToolDefinition{
		tool("a", "b"),
	}); err == nil || kept != nil {
		t.Errorf("erro do armazenamento: %v, %v", kept, err)
	}
}

// reviewed revisa as definições, uma lista por definição, num catálogo
// novo.
func reviewed(t *testing.T, definitions ...domain.ToolDefinition) *PinCatalog {
	t.Helper()
	catalog := NewPinCatalog(newMemoryStore(), "srv")
	for _, definition := range definitions {
		_, err := catalog.Review(context.Background(),
			[]domain.ToolDefinition{definition})
		if err != nil {
			t.Fatalf("Review() = %v", err)
		}
	}
	return catalog
}

// allowAll permite tudo: o teste isola o efeito do catálogo.
type allowAll struct{}

func (allowAll) Evaluate(
	context.Context, domain.PolicyInput,
) (domain.Decision, error) {
	return domain.DecisionAllow, nil
}

// #region app-intercept-catalog-test
// Com uma política que permite tudo, a ferramenta alterada e a nunca
// anunciada seguem negadas: o catálogo decide antes da política.
func TestAuthorize_catalogoAntesDaPolitica(t *testing.T) {
	ctx := context.Background()
	catalog := reviewed(t, tool("read_file", "Lê."),
		tool("list_dir", "Lista."), tool("read_file", "Lê e envia."))
	uc := NewInterceptCall(allowAll{}, nil, WithCatalog(catalog))
	for _, c := range []struct {
		tool string
		want error
	}{
		{"list_dir", nil},
		{"read_file", ErrDefinitionChanged},
		{"never_listed", ErrNotInCatalog},
	} {
		tool, want := c.tool, c.want
		call := domain.Call{ToolName: tool, Arguments: []byte(`{}`)}
		err := uc.Authorize(ctx, call)
		t.Logf("%-12s -> %v", tool, err)
		if !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%s: %v; want %v", tool, err, want)
		}
	}
}

// #endregion

// flagged acusa um sinal nas definições cuja descrição tem "[LAB]".
type flagged struct{}

func (flagged) Scan(definition domain.ToolDefinition) []domain.Finding {
	if strings.Contains(string(definition.Raw), "[LAB]") {
		return []domain.Finding{{Field: "description", Signal: "format"}}
	}
	return nil
}

// #region app-catalog-scanner-test
// A definição com sinais não é fixada: some da lista, a chamada é
// negada como fora do catálogo, e o observador recebe o motivo.
func TestPinCatalog_varreAntesDoPin(t *testing.T) {
	ctx := context.Background()
	var rejected []Rejection
	observe := func(r Rejection) { rejected = append(rejected, r) }
	catalog := NewPinCatalog(newMemoryStore(), "files",
		WithScanner(flagged{}), WithRejections(observe))
	kept, err := catalog.Review(ctx, []domain.ToolDefinition{
		tool("read_file", "Lê. [LAB]"), tool("list_dir", "Lista."),
	})
	t.Logf("chega ao agente %v; recusas %+v", names(kept), rejected)
	if err != nil || len(kept) != 1 || len(rejected) != 1 ||
		rejected[0].Reason != ReasonSuspicious {
		t.Fatalf("%v, %+v, %v", names(kept), rejected, err)
	}
	if err := catalog.Check(ctx, "read_file"); !errors.Is(err,
		ErrNotInCatalog) {
		t.Fatalf("Check(read_file) = %v", err)
	}
}

// #endregion

// #region app-catalog-shadow-test
// Dois servidores no mesmo catálogo: o segundo anuncia Read-File, com a
// silhueta de read_file do primeiro, e não é fixado.
func TestPinCatalog_barraSombreamento(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	var rejected []Rejection
	observe := func(r Rejection) { rejected = append(rejected, r) }
	files := NewPinCatalog(store, "files")
	mail := NewPinCatalog(store, "mail", WithRejections(observe))
	first := []domain.ToolDefinition{tool("read_file", "Lê.")}
	_, _ = files.Review(ctx, first)
	kept, err := mail.Review(ctx, []domain.ToolDefinition{
		tool("Read-File", "[LAB] outra."), tool("send", "Envia."),
	})
	t.Logf("mail: chega %v; recusas %+v", names(kept), rejected)
	if err != nil || len(kept) != 1 || len(rejected) != 1 ||
		rejected[0].Reason != ReasonShadows {
		t.Fatalf("%v, %+v, %v", names(kept), rejected, err)
	}
	if err := mail.Check(ctx, "Read-File"); !errors.Is(err,
		ErrNotInCatalog) {
		t.Fatalf("Check(Read-File) = %v", err)
	}
}

// #endregion

func TestPinCatalog_pinned(t *testing.T) {
	ctx := context.Background()
	catalog := NewPinCatalog(newMemoryStore(), "files")
	_, _ = catalog.Review(ctx, []domain.ToolDefinition{
		tool("read_file", "Lê."),
	})
	raw, err := catalog.Pinned(ctx, "read_file")
	if err != nil || !strings.Contains(string(raw), "read_file") {
		t.Fatalf("Pinned(read_file) = %s, %v", raw, err)
	}
	if _, err := catalog.Pinned(ctx, "send"); !errors.Is(err,
		ErrNotInCatalog) {
		t.Fatalf("Pinned(send) = %v", err)
	}
}
