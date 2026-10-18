package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// ErrMount indica uma montagem que exporia o host.
var ErrMount = errors.New("sandbox: mount not allowed")

// system são os diretórios do host que nenhum servidor vê: o sistema,
// os processos, os dispositivos e /run, onde fica o socket do Docker,
// que dá controle do host a quem o alcança.
var system = []string{
	"/etc", "/proc", "/sys", "/dev", "/boot", "/root",
	"/run", "/var/run",
}

// secrets são os diretórios do usuário com credenciais.
var secrets = []string{
	".ssh", ".aws", ".gnupg", ".config", ".docker",
	".kube",
}

// #region sandbox-check-mount
// CheckMount resolve a origem da montagem, seguindo cada symlink, e
// confere o caminho real. Ele não pode ficar num diretório do sistema
// ou de credenciais, nem conter um deles; e pode ficar dentro da home,
// mas não contê-la inteira. A montagem devolvida tem a origem
// resolvida, e é ela que vai para o Docker.
func CheckMount(m Mount, home string) (Mount, error) {
	if !filepath.IsAbs(m.Source) || !filepath.IsAbs(m.Target) {
		return m, fmt.Errorf("%w: paths must be absolute", ErrMount)
	}
	real, err := filepath.EvalSymlinks(m.Source)
	if err != nil {
		return m, fmt.Errorf("%w: %w", ErrMount, err)
	}
	for _, denied := range forbidden(home) {
		if within(real, denied) || within(denied, real) {
			return m, fmt.Errorf("%w: %s exposes %s", ErrMount, real,
				denied)
		}
	}
	if home != "" && within(home, real) {
		return m, fmt.Errorf("%w: %s contains the home", ErrMount, real)
	}
	m.Source, m.Target = real, filepath.Clean(m.Target)
	return m, nil
}

// #endregion

// forbidden junta os diretórios do sistema e os de credenciais da
// home.
func forbidden(home string) []string {
	out := slices.Clone(system)
	if home != "" {
		for _, dir := range secrets {
			out = append(out, filepath.Join(home, dir))
		}
	}
	return out
}

// #region sandbox-within
// within diz se path é dir ou fica dentro dele. A comparação é por
// componente, e não por prefixo de texto: /etcetera não fica em /etc.
// A raiz contém tudo.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// #endregion
