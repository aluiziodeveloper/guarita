package naiveops

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #region examples-naiveops-test
// Na porta do serviço, além de /mcp, respondem o índice dos perfis e a
// linha de comando do processo, com os argumentos que ele recebeu.
func TestHandler_expoePprof(t *testing.T) {
	server := httptest.NewServer(Handler())
	t.Cleanup(server.Close)
	for _, path := range []string{
		"/mcp", "/debug/pprof/", "/debug/pprof/cmdline",
	} {
		res, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		first, _, _ := strings.Cut(string(body), "\n")
		t.Logf("%-21s %d %.60q", path, res.StatusCode, first)
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
}

// #endregion
