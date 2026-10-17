package app

import (
	"context"
	"errors"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Desafio do capítulo 9: o operador revisa a definição divergente e a
// aprova. ApproveUpdate fixa a definição vista por último e tira a
// ferramenta do estado divergente; a antiga passa a ser a que diverge.
// Aprovar uma ferramenta que não mudou é ErrNotChanged.
func TestPinCatalog_aprovaAtualizacao(t *testing.T) {
	ctx := context.Background()
	original := tool("read_file", "Lê.")
	updated := tool("read_file", "Lê arquivos de texto.")
	catalog := reviewed(t, original, tool("list_dir", "Lista."), updated)

	if err := catalog.ApproveUpdate(ctx, "read_file"); err != nil {
		t.Fatalf("ApproveUpdate(read_file) = %v", err)
	}
	if err := catalog.Check(ctx, "read_file"); err != nil {
		t.Fatalf("Check depois da aprovação: %v", err)
	}
	kept, _ := catalog.Review(ctx, []domain.ToolDefinition{updated})
	if len(kept) != 1 {
		t.Fatalf("a definição aprovada não passou: %v", names(kept))
	}
	kept, _ = catalog.Review(ctx, []domain.ToolDefinition{original})
	if len(kept) != 0 {
		t.Fatalf("a definição antiga voltou a passar: %v", names(kept))
	}
	if err := catalog.ApproveUpdate(ctx, "list_dir"); !errors.Is(err,
		ErrNotChanged) {
		t.Errorf("list_dir não mudou: %v; want ErrNotChanged", err)
	}
	if err := catalog.ApproveUpdate(ctx, "never_listed"); !errors.Is(err,
		ErrNotInCatalog) {
		t.Errorf("fora do catálogo: %v; want ErrNotInCatalog", err)
	}
}
