package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// layout monta uma home falsa, com .ssh e um projeto, os dados fora
// dela e um symlink de nome inocente para o .ssh.
func layout(t *testing.T) (home, data, inside, link string) {
	t.Helper()
	dir := t.TempDir()
	home = filepath.Join(dir, "home", "operador")
	data = filepath.Join(dir, "projeto", "dados")
	inside = filepath.Join(home, "projeto")
	for _, d := range []string{
		filepath.Join(home, ".ssh"), data,
		inside,
	} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	link = filepath.Join(dir, "projeto", "relatorios")
	if err := os.Symlink(filepath.Join(home, ".ssh"), link); err != nil {
		t.Fatal(err)
	}
	return home, data, inside, link
}

// #region sandbox-check-mount-test
// Os dados, fora ou dentro da home, passam; a home, o pai dela, o .ssh
// pelo symlink de nome inocente, o socket do Docker e um caminho
// relativo, não.
func TestCheckMount(t *testing.T) {
	home, data, inside, link := layout(t)
	for _, c := range []struct {
		name, source string
		ok           bool
	}{
		{"dados", data, true},
		{"dentro da home", inside, true},
		{"home", home, false},
		{"pai da home", filepath.Dir(home), false},
		{"symlink", link, false},
		{"docker.sock", "/var/run", false},
		{"relativo", "projeto/dados", false},
	} {
		got, err := CheckMount(Mount{Source: c.source, Target: "/data"},
			home)
		t.Logf("%-14s %v", c.name, err)
		if (err == nil) != c.ok || (err != nil && !errors.Is(err,
			ErrMount)) || (c.ok && got.Source != c.source) {
			t.Errorf("%s: %v %v", c.name, got, err)
		}
	}
}

// #endregion

func TestWithin(t *testing.T) {
	for _, c := range []struct {
		path, dir string
		want      bool
	}{
		{"/etc", "/etc", true},
		{"/etc/ssh", "/etc", true},
		{"/etcetera", "/etc", false},
		{"/srv", "/", true},
		{"/", "/etc", false},
	} {
		if got := within(c.path, c.dir); got != c.want {
			t.Errorf("within(%q, %q) = %v", c.path, c.dir, got)
		}
	}
}
