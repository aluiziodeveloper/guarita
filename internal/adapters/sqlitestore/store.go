// Package sqlitestore guarda o catálogo da Guarita em SQLite, num único
// arquivo, com migrações embutidas no binário.
package sqlitestore

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"

	_ "modernc.org/sqlite" // registra o driver "sqlite", sem cgo
)

// #region sqlitestore-migrations-embed
// migrations são os arquivos .sql, compilados dentro do binário: a
// versão do esquema anda junto com a do código que o usa.
//
//go:embed migrations/*.sql
var migrations embed.FS

// #endregion

var _ app.CatalogStore = (*Store)(nil)

// Store é o catálogo em SQLite.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// #region sqlitestore-open
// Open abre ou cria o banco em path e aplica as migrações pendentes.
// Uma conexão só: o SQLite tem um escritor por vez, e o catálogo é
// pequeno. busy_timeout espera um bloqueio em vez de falhar na hora.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, allMigrations)
}

// allMigrations pede a migrate que aplique todas as pendentes.
const allMigrations = -1

func open(ctx context.Context, path string, upTo int) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db, upTo); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &Store{db: db, now: time.Now}, nil
}

// #endregion

// Close fecha o banco.
func (s *Store) Close() error { return s.db.Close() }

// #region sqlitestore-migrate
// migrate aplica, em ordem, as migrações acima de PRAGMA user_version,
// até upTo. Cada uma roda numa transação com o novo número: ou as duas
// coisas acontecem, ou nenhuma.
func migrate(ctx context.Context, db *sql.DB, upTo int) error {
	var current int
	if err := db.QueryRowContext(ctx,
		"PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	if upTo == allMigrations {
		upTo = len(files)
	}
	for version := current + 1; version <= upTo; version++ {
		script, err := migrations.ReadFile(files[version-1])
		if err != nil {
			return err
		}
		if err := apply(ctx, db, version, string(script)); err != nil {
			return fmt.Errorf("migration %s: %w", files[version-1], err)
		}
	}
	return nil
}

// #endregion

func apply(ctx context.Context, db *sql.DB, version int, script string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // sem efeito depois do Commit
	if _, err := tx.ExecContext(ctx, script); err != nil {
		return err
	}
	// PRAGMA não aceita parâmetro; version é um inteiro nosso.
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}

// #region sqlitestore-pin
// Pin lê a definição fixada. Os valores vão como parâmetros (?), nunca
// colados no texto da consulta: o nome da ferramenta vem do servidor.
func (s *Store) Pin(ctx context.Context, server, tool string) (
	domain.Pin, bool, error,
) {
	pin := domain.Pin{Server: server, Tool: tool}
	var definition string
	var seenFingerprint, seenDefinition sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint, definition,
		changed, seen_fingerprint, seen_definition
		FROM pins WHERE server = ? AND tool = ?`, server, tool).
		Scan(&pin.Fingerprint, &definition, &pin.Changed,
			&seenFingerprint, &seenDefinition)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Pin{}, false, nil
	}
	if err != nil {
		return domain.Pin{}, false, err
	}
	pin.Definition = []byte(definition)
	pin.SeenFingerprint = domain.Fingerprint(seenFingerprint.String)
	if seenDefinition.Valid {
		pin.SeenDefinition = []byte(seenDefinition.String)
	}
	return pin, true, nil
}

// #endregion

// #region sqlitestore-save-pin
// SavePin fixa uma definição nova. Se outra chamada fixou a mesma
// ferramenta antes, vale a primeira, e a revisão seguinte a compara.
func (s *Store) SavePin(ctx context.Context, pin domain.Pin) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO pins
		(server, tool, fingerprint, definition, pinned_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (server, tool) DO NOTHING`,
		pin.Server, pin.Tool, pin.Fingerprint, string(pin.Definition),
		s.now().UTC().Format(time.RFC3339))
	return err
}

// #endregion

// #region sqlitestore-mark-changed
// MarkChanged registra a definição divergente sem tocar na fixada: o
// operador precisa das duas para decidir.
func (s *Store) MarkChanged(
	ctx context.Context, server, tool string,
	seen domain.Fingerprint, definition []byte,
) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pins SET changed = 1,
		seen_fingerprint = ?, seen_definition = ?, changed_at = ?
		WHERE server = ? AND tool = ?`,
		seen, string(definition), s.now().UTC().Format(time.RFC3339),
		server, tool)
	return err
}

// #endregion
