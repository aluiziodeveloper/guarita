package stripauth

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"testing"
)

const userToken = "tok-do-usuario"

// leaked devolve os cabeçalhos que chegaram ao servidor com o token do
// usuário dentro.
func leaked(
	t *testing.T, newProxy func(*url.URL) *httputil.ReverseProxy,
) []string {
	t.Helper()
	got := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) { got <- r.Header }))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	proxy := httptest.NewServer(newProxy(target))
	t.Cleanup(proxy.Close)

	r, _ := http.NewRequest("POST", proxy.URL, strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+userToken)
	r.Header.Set("X-Api-Key", userToken)
	r.Header.Set("Cookie", "session="+userToken)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = response.Body.Close()
	var names []string
	for name, values := range <-got {
		if strings.Contains(strings.Join(values, ","), userToken) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// #region examples-stripauth-test
func TestProxy_repassaOTokenDoUsuario(t *testing.T) {
	for _, c := range []struct {
		name     string
		newProxy func(*url.URL) *httputil.ReverseProxy
	}{
		{"Capítulo 7", Unchanged},
		{"sem Authorization", StripAuthorization},
	} {
		names := leaked(t, c.newProxy)
		t.Logf("%-17s -> token do usuário em %d cabeçalhos: %v",
			c.name, len(names), names)
		if len(names) == 0 {
			t.Errorf("%s: nenhum vazamento", c.name)
		}
	}
}

// #endregion
