package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ErrAuditFailed indica que a decisão não pôde ser registrada; a
// chamada é negada.
var ErrAuditFailed = errors.New("audit record failed")

// #region app-auditor
// Auditor registra cada decisão do caso de uso. Os argumentos vão
// junto para que o adaptador calcule a prova deles, e não para que
// os grave.
type Auditor interface {
	Record(
		ctx context.Context, entry domain.AuditEntry,
		arguments json.RawMessage,
	) error
}

// WithAuditor liga a trilha de auditoria.
func WithAuditor(auditor Auditor) Option {
	return func(uc *InterceptCall) { uc.auditor = auditor }
}

// #endregion

// #region app-audit
// audit registra a decisão final. Uma chamada que passou por aprovação
// diz isso no motivo. Se o registro falhar, a chamada é negada, mesmo
// que tivesse sido permitida: sem evidência, a Guarita não age.
func (uc *InterceptCall) audit(
	ctx context.Context, call domain.Call, decided domain.Decision,
	err error,
) error {
	if uc.auditor == nil {
		return err
	}
	entry := domain.AuditEntry{
		Agent: uc.agent(ctx), Server: uc.server, Tool: call.ToolName,
		Decision: domain.DecisionAllow,
	}
	if decided == domain.DecisionRequireApproval {
		entry.Reason = "aprovada"
	}
	if err != nil {
		entry.Decision, entry.Reason = domain.DecisionDeny, err.Error()
	}
	rerr := uc.auditor.Record(ctx, entry, call.Arguments)
	if rerr != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, rerr)
	}
	return err
}

// #endregion
