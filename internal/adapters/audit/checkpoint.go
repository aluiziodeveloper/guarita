package audit

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// Erros do checkpoint.
var (
	ErrCheckpoint = errors.New("audit: trail does not match the checkpoint")
	ErrSignature  = errors.New("audit: checkpoint signature is not valid")
)

// #region audit-checkpoint
// Checkpoint é a cabeça da trilha num momento, assinada: o número e o
// hash de uma entrada. Guardado fora da máquina, ele acusa a cadeia
// refeita e o fim cortado, que a cadeia sozinha não acusa, e a
// assinatura impede que quem adulterou escreva um checkpoint novo.
type Checkpoint struct {
	Seq       uint64 `json:"seq"`
	Head      string `json:"head"`
	Signature string `json:"signature"`
}

// message é o que é assinado. O prefixo separa esta assinatura de
// qualquer outra feita com a mesma chave.
func message(seq uint64, head string) []byte {
	const domainTag = "guarita-audit-checkpoint"
	return fmt.Appendf(nil, "%s\n%d\n%s", domainTag, seq, head)
}

// Sign assina a entrada dada com a chave privada do operador.
func Sign(key ed25519.PrivateKey, e domain.AuditEntry) Checkpoint {
	sig := ed25519.Sign(key, message(e.Seq, e.Hash))
	return Checkpoint{e.Seq, e.Hash, hex.EncodeToString(sig)}
}

// #endregion

// #region audit-verify-against
// VerifyAgainst confere a assinatura do checkpoint e, lendo a trilha, a
// cadeia e a entrada do checkpoint: ela precisa existir e ter o hash
// assinado. As entradas depois dela são as que vieram depois.
func VerifyAgainst(
	r io.Reader, key ed25519.PublicKey, cp Checkpoint,
) error {
	sig, err := hex.DecodeString(cp.Signature)
	signed := message(cp.Seq, cp.Head)
	if err != nil || !ed25519.Verify(key, signed, sig) {
		return ErrSignature
	}
	found := false
	if _, err := verifyEach(r, func(e domain.AuditEntry) {
		found = found || (e.Seq == cp.Seq && e.Hash == cp.Head)
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: entry %d", ErrCheckpoint, cp.Seq)
	}
	return nil
}

// #endregion
