package app

import (
	"context"
	"errors"
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

func (m *memoryStore) Approve(_ context.Context, server, tool string) error {
	pin := m.pins[[2]string{server, tool}]
	pin.Fingerprint, pin.Definition = pin.SeenFingerprint, pin.SeenDefinition
	pin.Changed, pin.SeenFingerprint, pin.SeenDefinition = false, "", nil
	m.pins[[2]string{server, tool}] = pin
	return m.fail
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
	context.Context, domain.Call,
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
