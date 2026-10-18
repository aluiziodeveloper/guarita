package dlp

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// #region dlp-for-all
// forAll confere uma propriedade em n casos gerados por um PCG de
// semente fixa: a mesma semente gera os mesmos casos, e uma falha diz
// em que caso aconteceu, para que ela se repita.
func forAll(t *testing.T, n int, property func(*rand.Rand) error) {
	t.Helper()
	r := rand.New(rand.NewPCG(13, 2026))
	for i := range n {
		if err := property(r); err != nil {
			t.Fatalf("caso %d (semente 13, 2026): %v", i, err)
		}
	}
}

// #endregion

// randomCPF gera nove dígitos que não são todos iguais e completa.
func randomCPF(r *rand.Rand) string {
	for {
		base := fmt.Sprintf("%09d", r.IntN(1_000_000_000))
		if strings.Count(base, base[:1]) != 9 {
			return withChecks(base)
		}
	}
}

// formats escreve o CPF de um dos três jeitos comuns.
func formats(r *rand.Rand, cpf string) string {
	switch r.IntN(3) {
	case 0:
		return cpf
	case 1:
		return cpf[:3] + "." + cpf[3:6] + "." + cpf[6:9] + "-" + cpf[9:]
	}
	return cpf[:9] + "-" + cpf[9:]
}

func inspectText(text string) domain.Inspection {
	arguments, _ := json.Marshal(map[string]string{"body": text})
	inspection, _ := New(DefaultActions(), 4.3).Inspect(
		context.Background(), domain.PolicyInput{Call: domain.Call{
			ToolName: "http_request", Arguments: arguments,
		}})
	return inspection
}

// #region dlp-properties-test
// Três propriedades, mil casos cada: todo CPF válido é achado, em
// qualquer formato e no meio de texto; mudar um verificador o
// invalida; e o relatório nunca carrega o número.
func TestPropriedades(t *testing.T) {
	forAll(t, 1000, func(r *rand.Rand) error {
		cpf := randomCPF(r)
		text := "pedido " + formats(r, cpf) + " confirmado"
		got := inspectText(text)
		if got.Decision != domain.DecisionRequireApproval {
			return fmt.Errorf("CPF não achado em %q", text)
		}
		if strings.Contains(fmt.Sprint(got.Findings), cpf[3:9]) {
			return fmt.Errorf("o relatório vazou o CPF")
		}
		wrong := []byte(cpf)
		wrong[10] = '0' + (wrong[10]-'0'+1+byte(r.IntN(9)))%10
		if ValidCPF(string(wrong)) {
			return fmt.Errorf("verificador trocado passou: %s", wrong)
		}
		return nil
	})
	t.Log("1000 casos gerados por propriedade, sem falha")
}

// #endregion
