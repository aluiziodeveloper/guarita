package poisoning

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/adapters/scanner"
	"github.com/aluiziodeveloper/guarita/internal/adapters/sqlitestore"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// corpus lê as definições de testdata, com o nome tirado do JSON.
func corpus(t *testing.T) map[string]domain.ToolDefinition {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	definitions := map[string]domain.ToolDefinition{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var head struct{ Name string }
		_ = json.Unmarshal(raw, &head)
		definitions[filepath.Base(file)] = domain.ToolDefinition{
			Name: head.Name, Raw: raw,
		}
	}
	return definitions
}

// review passa uma definição por um catálogo novo e devolve a recusa.
func review(
	t *testing.T, definition domain.ToolDefinition, options ...app.CatalogOption,
) *app.Rejection {
	t.Helper()
	store, err := sqlitestore.Open(context.Background(),
		filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var rejected *app.Rejection
	options = append(options, app.WithRejections(
		func(r app.Rejection) { rejected = &r }))
	catalog := app.NewPinCatalog(store, "files", options...)
	_, err = catalog.Review(context.Background(),
		[]domain.ToolDefinition{definition})
	if err != nil {
		t.Fatal(err)
	}
	return rejected
}

// #region attacks-poisoning-test
// Sem o varredor, todo o corpus é fixado. Com ele, cada ataque é
// recusado pelo sinal do nome do arquivo, e nenhum caso limpo é.
func TestPoisoning(t *testing.T) {
	for file, definition := range corpus(t) {
		want, _, _ := strings.Cut(file, "-")
		if rejected := review(t, definition); rejected != nil {
			t.Errorf("%s: recusado sem o varredor: %+v", file, rejected)
		}
		rejected := review(t, definition,
			app.WithScanner(scanner.Structural{}))
		switch {
		case want == "clean" && rejected != nil:
			t.Errorf("%s: falso positivo: %+v", file, rejected.Findings)
		case want != "clean" && !hasSignal(rejected, want):
			t.Errorf("%s: sem o sinal %s: %+v", file, want, rejected)
		}
	}
}

// #endregion

func hasSignal(rejected *app.Rejection, signal string) bool {
	if rejected == nil {
		return false
	}
	for _, finding := range rejected.Findings {
		if finding.Signal == signal {
			return true
		}
	}
	return false
}

// TestPoisoning_relatorio imprime o que o varredor achou em cada caso.
func TestPoisoning_relatorio(t *testing.T) {
	files := make([]string, 0)
	definitions := corpus(t)
	for file := range definitions {
		files = append(files, file)
	}
	slices.Sort(files)
	for _, file := range files {
		found := scanner.Structural{}.Scan(definitions[file])
		var signals []string
		for _, f := range found {
			signals = append(signals, f.Signal+" "+f.Detail+" em "+f.Field)
		}
		t.Logf("%-34s %v", file, signals)
	}
}
