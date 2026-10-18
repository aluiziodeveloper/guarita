package threatmodel

import (
	"errors"
	"testing"
)

func TestModel_cadaAmeacaEValidaEUnica(t *testing.T) {
	seen := make(map[string]bool)
	for _, threat := range Model() {
		if err := threat.Validate(); err != nil {
			t.Errorf("Validate() = %v", err)
		}
		if seen[threat.ID] {
			t.Errorf("ID repetido: %q", threat.ID)
		}
		seen[threat.ID] = true
	}
	if got := len(seen); got != 14 {
		t.Errorf("o modelo tem %d ameaças; want 14", got)
	}
}

// #region threatmodel-stride-coverage-test
// Cada categoria STRIDE precisa aparecer em pelo menos uma ameaça: uma
// categoria vazia é uma pergunta que o modelo esqueceu de fazer.
func TestModel_cobreTodasAsCategoriasSTRIDE(t *testing.T) {
	covered := make(map[STRIDECategory]bool)
	for _, threat := range Model() {
		for _, category := range threat.Categories {
			covered[category] = true
		}
	}
	first, last := STRIDESpoofing, STRIDEElevationOfPrivilege
	for category := first; category <= last; category++ {
		if !covered[category] {
			t.Errorf("nenhuma ameaça cobre %s", category)
		}
	}
}

// #endregion

func TestModel_devolveCopiaNova(t *testing.T) {
	first := Model()
	status := first[0].Status
	first[0].Status = ThreatStatusMitigated
	first[0].Categories[0] = STRIDERepudiation

	second := Model()
	if second[0].Status != status {
		t.Errorf("Status = %s; a mudança vazou entre chamadas",
			second[0].Status)
	}
	if second[0].Categories[0] != STRIDETampering {
		t.Errorf("Categories[0] = %s; a mudança vazou entre chamadas",
			second[0].Categories[0])
	}
}

func TestThreatValidate_recusaAmeacaIncompleta(t *testing.T) {
	valid := Model()[0]
	tests := []struct {
		name   string
		change func(*Threat)
		kind   error
	}{
		{"ID com maiúscula", func(t *Threat) { t.ID = "Tool" }, ErrInvalidThreat},
		{"sem nome", func(t *Threat) { t.Name = "" }, ErrInvalidThreat},
		{"sem categoria", func(t *Threat) { t.Categories = nil }, ErrInvalidThreat},
		{
			"categoria zero",
			func(t *Threat) { t.Categories = []STRIDECategory{0} },
			ErrInvalidSTRIDECategory,
		},
		{
			"fronteira na mesma zona",
			func(t *Threat) { t.Boundary.To = t.Boundary.From },
			ErrInvalidTrustBoundary,
		},
		{"capítulo 23", func(t *Threat) { t.DefenseChapter = 23 }, ErrInvalidThreat},
		{"estado zero", func(t *Threat) { t.Status = 0 }, ErrInvalidThreatStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			threat := valid
			threat.Categories = append([]STRIDECategory(nil),
				valid.Categories...)
			test.change(&threat)
			if err := threat.Validate(); !errors.Is(err, test.kind) {
				t.Fatalf("Validate() = %v; want %v", err, test.kind)
			}
		})
	}
}
