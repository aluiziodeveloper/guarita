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
	if version != 2 || !found {
		t.Fatalf("user_version = %d, pin encontrado = %v", version, found)
	}
}

// #region sqlitestore-migration-test
// Um banco criado só com a 0001, com um pin, recebe a 0002 ao abrir: o
// pin continua lá, sem marca, e passa a aceitar a definição divergente.
func TestOpen_migraSobreDadosExistentes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	old, err := open(ctx, path, 1)
	if err != nil {
		t.Fatalf("abrir na 0001: %v", err)
	}
	_ = old.SavePin(ctx, pin("read_file", "sha256:aaa"))
	_ = old.Close()

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("migrar para a 0002: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	before, found, _ := store.Pin(ctx, server, "read_file")
	t.Logf("depois da 0002: encontrado=%v changed=%v", found,
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
