package domain

// #region domain-inspection
// Inspection é o que um inspetor conclui sobre uma chamada: os achados
// e a decisão que eles pedem. Sem achados, a decisão é allow. Um
// achado nunca traz o dado sensível em si, só onde e o quê.
type Inspection struct {
	Decision Decision
	Findings []Finding
}

// severity ordena as decisões: negar pesa mais que pedir aprovação, e
// pedir aprovação, mais que permitir.
var severity = map[Decision]int{
	DecisionAllow: 0, DecisionRequireApproval: 1, DecisionDeny: 2,
}

// Stricter devolve a mais severa das duas. Uma decisão fora do
// conjunto conta como negação: na dúvida, a Guarita fecha.
func Stricter(a, b Decision) Decision {
	for _, d := range []Decision{a, b} {
		if _, known := severity[d]; !known {
			return DecisionDeny
		}
	}
	if severity[b] > severity[a] {
		return b
	}
	return a
}

// #endregion
