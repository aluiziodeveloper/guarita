package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// recorder anota o próprio nome em order a cada inspeção, nos dois
// sentidos, e permite tudo.
type recorder struct {
	name  string
	order *[]string
}

func (r recorder) Inspect(
	context.Context, domain.PolicyInput,
) (domain.Inspection, error) {
	*r.order = append(*r.order, r.name)
	return domain.Inspection{Decision: domain.DecisionAllow}, nil
}

func (r recorder) InspectResult(
	context.Context, domain.PolicyInput, json.RawMessage,
) (domain.Inspection, error) {
	return r.Inspect(context.Background(), domain.PolicyInput{})
}

type marker struct{}

func (marker) Mark(
	_ domain.PolicyInput, result json.RawMessage,
) (json.RawMessage, error) {
	return result, nil
}

type allowAll struct{}

func (allowAll) Evaluate(
	context.Context, domain.PolicyInput,
) (domain.Decision, error) {
	return domain.DecisionAllow, nil
}

// registry tem duas etapas de ida, duas de volta e o marcador.
func registry(order *[]string) Registry {
	call := func(name string) Factory {
		return func() (Stage, error) {
			return Stage{Call: recorder{name, order}}, nil
		}
	}
	result := func(name string) Factory {
		return func() (Stage, error) {
			return Stage{Result: recorder{name, order}}, nil
		}
	}
	return Registry{
		"a": call("a"), "b": call("b"),
		"x": result("x"), "y": result("y"),
		"mark": func() (Stage, error) {
			return Stage{Marker: marker{}}, nil
		},
		"needs-catalog": func() (Stage, error) {
			return Stage{}, errors.New("requires -catalog")
		},
		"empty": func() (Stage, error) { return Stage{}, nil },
	}
}

// #region pipeline-test-order
// A ordem escrita é a ordem em que o caso de uso consulta as etapas,
// em cada sentido.
func TestBuild_ordemEscrita(t *testing.T) {
	for _, names := range [][]string{
		{"a", "b", "x", "y", "mark"}, {"b", "a", "y", "x"},
	} {
		var order []string
		p, err := registry(&order).Build(names)
		if err != nil {
			t.Fatal(err)
		}
		uc := app.NewInterceptCall(allowAll{}, nil, p.Options()...)
		call := domain.Call{ToolName: "t", Arguments: []byte(`{}`)}
		_ = uc.Authorize(t.Context(), call)
		_, _ = uc.ReviewResult(t.Context(), call, []byte(`{}`))
		t.Logf("%v -> %v", names, order)
		want := strings.TrimSuffix(strings.Join(names, " "), " mark")
		if got := strings.Join(order, " "); got != want {
			t.Errorf("ordem %q; quer %q", got, want)
		}
	}
}

// #endregion

// #region pipeline-test-errors
// Cada erro de montagem recusa a rota inteira, com o nome da etapa.
func TestBuild_erros(t *testing.T) {
	var order []string
	for _, c := range []struct {
		names []string
		want  error
	}{
		{[]string{"a", "dlp"}, ErrUnknownStage},
		{[]string{"a", "x", "a"}, ErrDuplicateStage},
		{[]string{"x", "mark", "y"}, ErrMarkerNotLast},
		{[]string{"empty"}, ErrEmptyStage},
	} {
		_, err := registry(&order).Build(c.names)
		t.Logf("%-12s %v", strings.Join(c.names, ","), err)
		if !errors.Is(err, c.want) {
			t.Errorf("%v: %v; quer %v", c.names, err, c.want)
		}
	}
	_, err := registry(&order).Build([]string{"needs-catalog"})
	if err == nil || !strings.Contains(err.Error(), "-catalog") {
		t.Errorf("erro da fábrica perdido: %v", err)
	}
}

// #endregion

func TestMissing(t *testing.T) {
	var order []string
	p, _ := registry(&order).Build([]string{"a", "x"})
	got := p.Missing([]string{"a", "b", "mark"})
	if strings.Join(got, ",") != "b,mark" {
		t.Errorf("Missing = %v", got)
	}
	if p.ReadsResults() != true {
		t.Error("rota com etapa de volta não lê resultados")
	}
	q, _ := registry(&order).Build([]string{"a"})
	if q.ReadsResults() {
		t.Error("rota só de ida lê resultados")
	}
}
