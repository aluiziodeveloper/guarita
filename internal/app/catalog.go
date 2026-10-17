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
}

// #endregion

// PinCatalog fixa as definições de um servidor na primeira vez que as
// vê e barra as que mudam depois.
type PinCatalog struct {
	store   CatalogStore
	server  string
	scanner DefinitionScanner
	observe func(Rejection)
}

// #region app-catalog-scanner
// DefinitionScanner procura sinais suspeitos numa definição. O
// varredor estrutural é um adaptador; os classificadores dos próximos
// capítulos serão outros.
type DefinitionScanner interface {
	Scan(definition domain.ToolDefinition) []domain.Finding
}

// Rejection explica por que uma definição não chegou ao agente.
type Rejection struct {
	Server, Tool, Reason string
	Findings             []domain.Finding
}

// Motivos de uma recusa.
const (
	ReasonInvalid    = "invalid"    // não é um objeto JSON com nome
	ReasonSuspicious = "suspicious" // o varredor achou sinais
	ReasonChanged    = "changed"    // divergiu da fixada agora
	ReasonMarked     = "marked"     // já estava marcada como alterada
)

// #endregion

// #region app-catalog-options
// CatalogOption ajusta o catálogo.
type CatalogOption func(*PinCatalog)

// WithScanner varre cada definição antes de qualquer decisão.
func WithScanner(scanner DefinitionScanner) CatalogOption {
	return func(uc *PinCatalog) { uc.scanner = scanner }
}

// WithRejections recebe cada recusa, para o registro do operador.
func WithRejections(observe func(Rejection)) CatalogOption {
	return func(uc *PinCatalog) { uc.observe = observe }
}

// #endregion

// NewPinCatalog cria o catálogo de um servidor.
func NewPinCatalog(
	store CatalogStore, server string, options ...CatalogOption,
) *PinCatalog {
	uc := &PinCatalog{store: store, server: server, observe: func(Rejection) {}}
	for _, option := range options {
		option(uc)
	}
	return uc
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
		rejection, err := uc.review(ctx, definition)
		if err != nil {
			return nil, fmt.Errorf("catalog %s: %w", uc.server, err)
		}
		if rejection.Reason == "" {
			kept = append(kept, definition)
			continue
		}
		rejection.Server, rejection.Tool = uc.server, definition.Name
		uc.observe(rejection)
	}
	return kept, nil
}

// #endregion

// #region app-catalog-review-one
// review decide uma definição; a com sinais suspeitos nem é fixada.
func (uc *PinCatalog) review(
	ctx context.Context, definition domain.ToolDefinition,
) (Rejection, error) {
	fingerprint, err := definition.Fingerprint()
	if err != nil {
		return Rejection{Reason: ReasonInvalid}, nil
	}
	if findings := uc.scan(definition); len(findings) > 0 {
		return suspicious(findings), nil
	}
	pin, found, err := uc.store.Pin(ctx, uc.server, definition.Name)
	switch {
	case err != nil:
		return Rejection{}, err
	case !found: // confiança no primeiro uso, após a varredura
		return Rejection{}, uc.store.SavePin(ctx, domain.Pin{
			Server: uc.server, Tool: definition.Name,
			Fingerprint: fingerprint, Definition: definition.Raw,
		})
	case pin.Changed:
		return Rejection{Reason: ReasonMarked}, nil
	case pin.Fingerprint != fingerprint:
		err := uc.store.MarkChanged(ctx, uc.server, definition.Name,
			fingerprint, definition.Raw)
		return Rejection{Reason: ReasonChanged}, err
	}
	return Rejection{}, nil
}

// #endregion

func suspicious(findings []domain.Finding) Rejection {
	return Rejection{Reason: ReasonSuspicious, Findings: findings}
}

func (uc *PinCatalog) scan(definition domain.ToolDefinition) []domain.Finding {
	if uc.scanner == nil {
		return nil
	}
	return uc.scanner.Scan(definition)
}

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
