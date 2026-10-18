package threatmodel

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidThreat identifica uma ameaça incompleta no modelo.
var ErrInvalidThreat = errors.New("threat is not valid")

var threatID = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

// #region threatmodel-threat
// Threat é uma linha do modelo de ameaças: o que pode dar errado, por
// onde entra e em que capítulo a Guarita ganha a defesa.
type Threat struct {
	ID             string
	Name           string
	Categories     []STRIDECategory
	Boundary       TrustBoundary
	DefenseChapter int
	Status         ThreatStatus
}

// #endregion

// Validate exige identificador estável, categorias, fronteira, capítulo
// e estado válidos.
func (threat Threat) Validate() error {
	if !threatID.MatchString(threat.ID) || threat.Name == "" {
		return fmt.Errorf("threat %q: %w", threat.ID, ErrInvalidThreat)
	}
	if len(threat.Categories) == 0 {
		return fmt.Errorf("threat %q: no STRIDE category: %w",
			threat.ID, ErrInvalidThreat)
	}
	for _, category := range threat.Categories {
		if err := category.Validate(); err != nil {
			return fmt.Errorf("threat %q: %w", threat.ID, err)
		}
	}
	if err := threat.Boundary.Validate(); err != nil {
		return fmt.Errorf("threat %q: %w", threat.ID, err)
	}
	if threat.DefenseChapter < 1 || threat.DefenseChapter > 22 {
		return fmt.Errorf("threat %q: chapter %d: %w",
			threat.ID, threat.DefenseChapter, ErrInvalidThreat)
	}
	if err := threat.Status.Validate(); err != nil {
		return fmt.Errorf("threat %q: %w", threat.ID, err)
	}
	return nil
}

// #region threatmodel-model
// Model devolve uma cópia nova do modelo a cada chamada, para que
// nenhum chamador altere o modelo visto por outro.
func Model() []Threat {
	in, out := TrustZoneMCPServer, TrustZoneGuarita
	agent, operator := TrustZoneAgent, TrustZoneOperator
	return []Threat{
		threat("tool-poisoning", "Envenenamento de ferramenta",
			edge(in, out), 10, tampering, elevationOfPrivilege,
		).with(ThreatStatusPartiallyMitigated),
		threat("rug-pull", "Troca silenciosa da definição aprovada",
			edge(in, out), 9, tampering,
		).with(ThreatStatusPartiallyMitigated),
		threat("tool-shadowing",
			"Sombreamento de ferramenta por nome repetido",
			edge(in, out), 10, spoofing,
		).with(ThreatStatusPartiallyMitigated),
		threat("indirect-prompt-injection",
			"Injeção de prompt indireta na resposta",
			edge(in, out), 14, tampering, elevationOfPrivilege,
		).with(ThreatStatusPartiallyMitigated),
		threat("excessive-agency", "Agência excessiva",
			edge(agent, out), 11, elevationOfPrivilege,
		).with(ThreatStatusPartiallyMitigated),
		threat("data-exfiltration",
			"Exfiltração de dados nos argumentos",
			edge(out, in), 13, informationDisclosure,
		).with(ThreatStatusPartiallyMitigated),
		threat("sandbox-escape", "Fuga da sandbox do servidor",
			edge(out, in), 17, elevationOfPrivilege,
		).with(ThreatStatusPartiallyMitigated),
		threat("credential-theft",
			"Repasse ou roubo de credenciais (confused deputy)",
			edge(out, in), 8, spoofing, informationDisclosure,
		).with(ThreatStatusPartiallyMitigated),
		threat("supply-chain", "Pacote de servidor comprometido",
			edge(operator, in), 20, tampering),
		threat("resource-abuse",
			"Laços de chamadas que esgotam cota ou custo",
			edge(agent, out), 18, denialOfService,
		).with(ThreatStatusPartiallyMitigated),
		threat("insecure-parsing",
			"Mensagem malformada ou ambígua no protocolo",
			edge(agent, out), 6, tampering, denialOfService,
		).with(ThreatStatusPartiallyMitigated),
		threat("approval-forgery",
			"Aprovação forjada ou obtida por fadiga",
			edge(operator, out), 12, spoofing, repudiation,
		).with(ThreatStatusPartiallyMitigated),
		threat("dns-rebinding",
			"Página da web alcança a Guarita local por DNS rebinding",
			edge(agent, out), 7, spoofing, elevationOfPrivilege,
		).with(ThreatStatusPartiallyMitigated),
	}
}

// #endregion

// Nomes curtos para as categorias, só para manter o modelo legível.
const (
	spoofing              = STRIDESpoofing
	tampering             = STRIDETampering
	repudiation           = STRIDERepudiation
	informationDisclosure = STRIDEInformationDisclosure
	denialOfService       = STRIDEDenialOfService
	elevationOfPrivilege  = STRIDEElevationOfPrivilege
)

func threat(
	id, name string,
	boundary TrustBoundary,
	chapter int,
	categories ...STRIDECategory,
) Threat {
	return Threat{
		ID:             id,
		Name:           name,
		Categories:     categories,
		Boundary:       boundary,
		DefenseChapter: chapter,
		Status:         ThreatStatusOpen,
	}
}

// with muda o estado de uma ameaça. Pela regra do ADR 0002, só com
// teste de ataque na suíte do capítulo que trouxe a defesa.
func (threat Threat) with(status ThreatStatus) Threat {
	threat.Status = status
	return threat
}

func edge(from, to TrustZone) TrustBoundary {
	return TrustBoundary{From: from, To: to}
}
