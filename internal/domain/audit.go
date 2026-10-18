package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erros da cadeia de auditoria. Cada um diz qual elo quebrou.
var (
	ErrAuditSeq  = errors.New("audit: sequence out of order")
	ErrAuditPrev = errors.New("audit: previous hash does not match")
	ErrAuditHash = errors.New("audit: entry hash does not match")
)

// Genesis é o hash "anterior" da primeira entrada da trilha.
var Genesis = strings.Repeat("0", 64)

// #region domain-audit-entry
// AuditEntry é uma linha da trilha de auditoria: quem pediu, em que
// servidor, qual ferramenta, o que foi decidido e por quê, e o elo com
// a linha anterior. Ela não guarda os argumentos: guarda um HMAC deles,
// que prova quais foram para quem tem a chave, sem revelá-los.
type AuditEntry struct {
	Seq      uint64    `json:"seq"`
	Time     time.Time `json:"time"`
	Agent    string    `json:"agent"`
	Server   string    `json:"server"`
	Tool     string    `json:"tool"`
	Decision Decision  `json:"decision"`
	Reason   string    `json:"reason,omitempty"`
	ArgsMAC  string    `json:"args_mac,omitempty"`
	TraceID  string    `json:"trace_id,omitempty"`
	Prev     string    `json:"prev"`
	Hash     string    `json:"hash"`
}

// #endregion

// #region domain-audit-seal
// Seal calcula o hash da entrada: SHA-256 do JSON dela com o campo Hash
// vazio. O JSON de uma estrutura sai sempre com os campos na ordem da
// declaração, e por isso o mesmo conteúdo dá sempre o mesmo hash. O
// tempo vai em UTC, sem a leitura monotônica, para sobreviver à ida e
// à volta pelo arquivo.
func (e AuditEntry) Seal() AuditEntry {
	e.Time, e.Hash = e.Time.UTC().Round(0), ""
	body, _ := json.Marshal(e) // só tipos simples: não falha
	sum := sha256.Sum256(body)
	e.Hash = hex.EncodeToString(sum[:])
	return e
}

// #endregion

// #region domain-audit-follows
// Follows confere que e é a entrada seguinte a prev na cadeia: o número
// é o próximo, o anterior é o hash de prev e o hash de e confere com o
// conteúdo dele. Para a primeira entrada, prev é o valor zero.
func (e AuditEntry) Follows(prev AuditEntry) error {
	wantSeq, wantPrev := prev.Seq+1, prev.Hash
	if prev.Hash == "" {
		wantSeq, wantPrev = 1, Genesis
	}
	switch {
	case e.Seq != wantSeq:
		return fmt.Errorf("%w: got %d, want %d", ErrAuditSeq, e.Seq,
			wantSeq)
	case e.Prev != wantPrev:
		return fmt.Errorf("%w at %d", ErrAuditPrev, e.Seq)
	case e.Seal().Hash != e.Hash:
		return fmt.Errorf("%w at %d", ErrAuditHash, e.Seq)
	}
	return nil
}

// #endregion
