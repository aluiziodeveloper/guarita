package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Erros do catálogo.
var (
	// ErrNotInCatalog nega a chamada a uma ferramenta que o servidor
	// nunca anunciou à Guarita.
	ErrNotInCatalog = errors.New("tool is not in the catalog")
	// ErrDefinitionChanged nega a chamada a uma ferramenta cuja
	// definição mudou depois de fixada.
	ErrDefinitionChanged = errors.New("tool definition changed since it was pinned")
	// ErrNotChanged recusa aprovar uma ferramenta que não divergiu.
	ErrNotChanged = errors.New("tool definition has not changed")
)

// #region app-catalog-store
// CatalogStore guarda as definições fixadas. found informa se a
// ferramenta já tem definição fixada naquele servidor.
type CatalogStore interface {
	Pin(ctx context.Context, server, tool string) (
		pin domain.Pin, found bool, err error)
	SavePin(ctx context.Context, pin domain.Pin) error
	MarkChanged(ctx context.Context, server, tool string,
		seen domain.Fingerprint, definition []byte) error
	// Approve troca a fixada pela vista por último e tira a marca.
	Approve(ctx context.Context, server, tool string) error
}

// #endregion

// PinCatalog fixa as definições de um servidor na primeira vez que as
// vê e barra as que mudam depois.
type PinCatalog struct {
	store  CatalogStore
	server string
}

// NewPinCatalog cria o catálogo de um servidor.
func NewPinCatalog(store CatalogStore, server string) *PinCatalog {
	return &PinCatalog{store: store, server: server}
}

// #region app-catalog-review
// Review devolve as definições que podem chegar ao agente. A nova fica
// fixada agora; a que diverge da fixada, ou a de uma ferramenta já
// marcada, fica de fora. Erro do armazenamento derruba a lista toda.
func (uc *PinCatalog) Review(
	ctx context.Context, definitions []domain.ToolDefinition,
) ([]domain.ToolDefinition, error) {
	var kept []domain.ToolDefinition
	for _, definition := range definitions {
		keep, err := uc.review(ctx, definition)
		if err != nil {
			return nil, fmt.Errorf("catalog %s: %w", uc.server, err)
		}
		if keep {
			kept = append(kept, definition)
		}
	}
	return kept, nil
}

// #endregion

// #region app-catalog-review-one
func (uc *PinCatalog) review(
	ctx context.Context, definition domain.ToolDefinition,
) (bool, error) {
	fingerprint, err := definition.Fingerprint()
	if err != nil {
		return false, nil // definição inválida não chega ao agente
	}
	pin, found, err := uc.store.Pin(ctx, uc.server, definition.Name)
	switch {
	case err != nil:
		return false, err
	case !found: // confiança no primeiro uso
		return true, uc.store.SavePin(ctx, domain.Pin{
			Server: uc.server, Tool: definition.Name,
			Fingerprint: fingerprint, Definition: definition.Raw,
		})
	case pin.Changed:
		return false, nil // continua barrada até o operador decidir
	case pin.Fingerprint != fingerprint:
		return false, uc.store.MarkChanged(ctx, uc.server,
			definition.Name, fingerprint, definition.Raw)
	}
	return true, nil
}

// #endregion

// #region app-catalog-check
// Check nega a chamada a uma ferramenta fora do catálogo ou alterada.
func (uc *PinCatalog) Check(ctx context.Context, tool string) error {
	pin, found, err := uc.store.Pin(ctx, uc.server, tool)
	switch {
	case err != nil:
		return fmt.Errorf("catalog %s: %w", uc.server, err)
	case !found:
		return ErrNotInCatalog
	case pin.Changed:
		return ErrDefinitionChanged
	}
	return nil
}

// #endregion

// ApproveUpdate aceita a definição divergente de tool, depois da
// revisão do operador: ela passa a ser a fixada.
func (uc *PinCatalog) ApproveUpdate(ctx context.Context, tool string) error {
	pin, found, err := uc.store.Pin(ctx, uc.server, tool)
	switch {
	case err != nil:
		return fmt.Errorf("catalog %s: %w", uc.server, err)
	case !found:
		return ErrNotInCatalog
	case !pin.Changed:
		return ErrNotChanged
	}
	return uc.store.Approve(ctx, uc.server, tool)
}
