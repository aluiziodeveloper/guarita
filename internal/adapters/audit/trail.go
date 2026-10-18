// Package audit grava a trilha de auditoria num arquivo JSON Lines,
// só de acréscimo, com cada entrada encadeada à anterior, e a verifica.
package audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

// ErrLine indica uma linha que não é a forma canônica de uma entrada.
var ErrLine = errors.New("audit: line is not a canonical entry")

// maxLine limita uma linha lida: uma entrada tem poucas centenas de
// bytes, e uma linha enorme é sinal de arquivo errado.
const maxLine = 64 << 10

// #region audit-trail
// Trail é a trilha aberta. A trava ordena as gravações: o número e o
// elo de cada entrada dependem da anterior, e duas chamadas ao mesmo
// tempo não podem encadear no mesmo elo.
type Trail struct {
	mu   sync.Mutex
	file *os.File
	head domain.AuditEntry // a última entrada gravada
	key  []byte            // chave do HMAC dos argumentos
}

// #endregion

// #region audit-open
// Open abre a trilha para acrescentar e verifica o que já está nela: a
// cadeia continua do último elo, e uma trilha quebrada não é estendida,
// porque as entradas novas herdariam a quebra sem que ninguém notasse.
func Open(path string, key []byte) (*Trail, error) {
	//nolint:gosec // G304: o caminho vem da configuração do operador
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	report, err := Verify(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("audit: existing trail: %w", err)
	}
	return &Trail{file: file, head: report.Last, key: key}, nil
}

// #endregion

// #region audit-record
// Record numera, carimba, encadeia e sela a entrada, grava uma linha
// e espera o disco confirmar, com Sync, antes de devolver: a chamada
// só segue depois de a decisão estar registrada. O HMAC dos argumentos
// é calculado aqui, e os argumentos não vão a lugar nenhum.
func (t *Trail) Record(
	ctx context.Context, e domain.AuditEntry, arguments json.RawMessage,
) error {
	e.TraceID = traceID(ctx)
	t.mu.Lock()
	defer t.mu.Unlock()
	e.Seq, e.Prev = t.head.Seq+1, t.head.Hash
	if t.head.Hash == "" {
		e.Prev = domain.Genesis
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	e.ArgsMAC = t.mac(arguments)
	sealed := e.Seal()
	line, _ := json.Marshal(sealed)
	if _, err := t.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	if err := t.file.Sync(); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	t.head = sealed
	return nil
}

// #endregion

// #region audit-mac
// mac é o HMAC-SHA256 dos argumentos, sem espaços, com a chave da
// trilha. Um hash simples não serviria: um CPF tem poucas
// possibilidades, e quem tivesse a trilha descobriria o valor testando
// todas.
func (t *Trail) mac(arguments json.RawMessage) string {
	if len(t.key) == 0 || len(arguments) == 0 {
		return ""
	}
	var compact bytes.Buffer
	if json.Compact(&compact, arguments) != nil {
		compact.Reset()
		compact.Write(arguments)
	}
	h := hmac.New(sha256.New, t.key)
	h.Write(compact.Bytes())
	return hex.EncodeToString(h.Sum(nil))
}

// #endregion

// canonical diz se a linha é a forma canônica da entrada. Só importa
// quando o hash confere: um hash errado, Follows já acusa. Um campo a
// mais, ou o mesmo JSON escrito de outro jeito, passaria pelo hash, que
// é calculado sobre a estrutura, e não sobre a linha.
func canonical(e domain.AuditEntry, line []byte) bool {
	sealed := e.Seal()
	if sealed.Hash != e.Hash {
		return true
	}
	want, _ := json.Marshal(sealed)
	return bytes.Equal(want, line)
}

// traceID é o trace do contexto, o mesmo do agente e do span da
// Guarita, ou vazio sem trace.
func traceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// Close fecha o arquivo.
func (t *Trail) Close() error { return t.file.Close() }

// Head devolve a última entrada gravada.
func (t *Trail) Head() domain.AuditEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.head
}

// Report é o resultado de uma verificação.
type Report struct {
	Entries int
	Last    domain.AuditEntry
	Since   time.Time // o tempo da primeira entrada
}

// #region audit-verify
// Verify lê a trilha do começo e confere cada linha: ela precisa ser,
// byte a byte, a forma canônica da entrada que descreve, e a entrada
// precisa seguir a anterior. A primeira quebra para a leitura, com o
// número da linha.
func Verify(r io.Reader) (Report, error) {
	lines := bufio.NewScanner(r)
	lines.Buffer(make([]byte, 4096), maxLine)
	var report Report
	for n := 1; lines.Scan(); n++ {
		var e domain.AuditEntry
		if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
			return report, fmt.Errorf("line %d: %w: %w", n, ErrLine,
				err)
		}
		if !canonical(e, lines.Bytes()) {
			return report, fmt.Errorf("line %d: %w", n, ErrLine)
		}
		if err := e.Follows(report.Last); err != nil {
			return report, fmt.Errorf("line %d: %w", n, err)
		}
		if n == 1 {
			report.Since = e.Time
		}
		report.Entries, report.Last = n, e
	}
	return report, lines.Err()
}

// #endregion
