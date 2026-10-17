package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aluiziodeveloper/guarita/internal/domain"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

const server = "http://files.lab.test/mcp"

// insertOld grava um pin com as colunas da 0001. O estado antigo vem
// em SQL: o código atual fala o esquema mais novo e falharia ali.
func insertOld(t *testing.T, store *Store, server, tool, fp string) {
	t.Helper()
	_, err := store.db.ExecContext(context.Background(), `INSERT INTO
		pins (server, tool, fingerprint, definition, pinned_at)
		VALUES (?, ?, ?, '{}', 'antes')`, server, tool, fp)
	if err != nil {
		t.Fatalf("gravar no esquema antigo: %v", err)
	}
}

func pin(tool, fingerprint string) domain.Pin {
	return domain.Pin{
		Server: server, Tool: tool,
		Fingerprint: domain.Fingerprint(fingerprint),
		Definition:  []byte(`{"name":"` + tool + `"}`),
	}
}

// #region sqlitestore-pin-test
// A primeira fixação vale; o nome que parece SQL é só um nome.
func TestStore_fixaUmaVezEGuardaComoTexto(t *testing.T) {
	store, _ := openTemp(t)
	ctx := context.Background()
	const injection = `x' OR '1'='1`
	for _, p := range []domain.Pin{
		pin("read_file", "sha256:aaa"), pin("read_file", "sha256:bbb"),
		pin(injection, "sha256:ccc"),
	} {
		if err := store.SavePin(ctx, p); err != nil {
			t.Fatalf("SavePin(%s) = %v", p.Tool, err)
		}
	}
	got, found, err := store.Pin(ctx, server, "read_file")
	if err != nil || !found || got.Fingerprint != "sha256:aaa" {
		t.Fatalf("read_file: %+v, %v, %v", got, found, err)
	}
	_, found, _ = store.Pin(ctx, server, "other")
	if found {
		t.Fatal("a injeção encontrou uma ferramenta que não existe")
	}
	got, found, _ = store.Pin(ctx, server, injection)
	if !found || got.Tool != injection {
		t.Fatalf("o nome não foi guardado como texto: %+v", got)
	}
}

// #endregion

func TestOpen_migracoesIdempotentes(t *testing.T) {
	store, path := openTemp(t)
	ctx := context.Background()
	if err := store.SavePin(ctx, pin("read_file", "sha256:aaa")); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	t.Cleanup(func() { _ = again.Close() })
	var version int
	_ = again.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
	_, found, _ := again.Pin(ctx, server, "read_file")
	if version != 3 || !found {
		t.Fatalf("user_version = %d, pin encontrado = %v", version, found)
	}
}

// #region sqlitestore-migration-test
// Um banco criado só com a 0001, com um pin, recebe as migrações novas
// ao abrir: o pin continua lá, sem marca, e aceita a divergente.
func TestOpen_migraSobreDadosExistentes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	old, err := open(ctx, path, 1)
	if err != nil {
		t.Fatalf("abrir na 0001: %v", err)
	}
	insertOld(t, old, server, "read_file", "sha256:aaa")
	_ = old.Close()

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("migrar: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	before, found, _ := store.Pin(ctx, server, "read_file")
	t.Logf("depois das migrações: encontrado=%v changed=%v", found,
		before.Changed)
	err = store.MarkChanged(ctx, server, "read_file", "sha256:bbb",
		[]byte(`{"name":"read_file","description":"nova"}`))
	after, _, _ := store.Pin(ctx, server, "read_file")
	if err != nil || !found || before.Changed || !after.Changed ||
		after.SeenFingerprint != "sha256:bbb" ||
		after.Fingerprint != "sha256:aaa" {
		t.Fatalf("antes %+v, depois %+v, err %v", before, after, err)
	}
}

// #endregion

// skeletonNames tem nomes válidos, maiúsculas, separadores e letras
// fora do ASCII, que lower não muda no SQLite.
var skeletonNames = []string{
	"read_file", "Read-File", "admin.Tools.LIST",
	"Ação-X", "ÇA.b",
}

// #region sqlitestore-skeleton-test
// A silhueta que a 0003 calcula em SQL é a de domain.Skeleton.
func TestOpen_silhuetaEmSQLIgualAoGo(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	old, err := open(ctx, path, 2)
	if err != nil {
		t.Fatalf("abrir na 0002: %v", err)
	}
	names := skeletonNames
	for _, name := range names {
		insertOld(t, old, "old", name, "f")
	}
	_ = old.Close()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("migrar para a 0003: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, name := range names {
		var got string
		_ = store.db.QueryRowContext(ctx, `SELECT skeleton FROM pins
			WHERE tool = ?`, name).Scan(&got)
		if want := domain.Skeleton(name); got != want {
			t.Errorf("%q: SQL %q, Go %q", name, got, want)
		}
	}
}

// #endregion

func TestStore_ownerAchaOutroServidor(t *testing.T) {
	store, _ := openTemp(t)
	ctx := context.Background()
	first := pin("read_file", "sha256:aaa")
	_ = store.SavePin(ctx, first)
	owner, found, err := store.Owner(ctx, "mail", "Read-File")
	if err != nil || !found || owner != server {
		t.Fatalf("Owner() = %q, %v, %v", owner, found, err)
	}
	if _, found, _ := store.Owner(ctx, server, "read_file"); found {
		t.Fatal("o próprio servidor não colide consigo mesmo")
	}
}
