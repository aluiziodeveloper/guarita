package mcphttp_test

import (
	"crypto/ed25519"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/aluiziodeveloper/guarita/internal/adapters/identity"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcpproxy"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
	"github.com/aluiziodeveloper/guarita/internal/domain"
	"github.com/aluiziodeveloper/guarita/internal/platform/secret"
)

const lab = "https://auth.lab.test"

// deputy monta a Guarita como a raiz de composição a monta: guarda de
// Host, identidade do agente e handler com a credencial da Guarita.
func deputy(t *testing.T, upstream string, key ed25519.PublicKey) string {
	t.Helper()
	target, _ := url.Parse(upstream + "/mcp")
	policy, _ := policystatic.New(map[string]domain.Decision{
		"read_file": domain.DecisionAllow,
	})
	gate := mcpproxy.NewGate(app.NewInterceptCall(policy, nil))
	handler, _ := mcphttp.NewHandler(target, gate,
		mcphttp.WithUpstreamCredential(secret.New("cred-da-guarita")))
	guarita := httptest.NewUnstartedServer(nil)
	resource := "http://" + guarita.Listener.Addr().String() + "/mcp"
	verifier, err := identity.NewVerifier(key, lab, resource)
	if err != nil {
		t.Fatalf("NewVerifier() = %v", err)
	}
	guard, _ := mcphttp.NewGuard(
		[]string{guarita.Listener.Addr().String()}, nil)
	guarita.Config.Handler = guard.Wrap(
		identity.Require(verifier, "")(handler))
	guarita.Start()
	t.Cleanup(guarita.Close)
	return resource
}

func token(t *testing.T, key ed25519.PrivateKey, audience string) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA,
		jwt.RegisteredClaims{
			Issuer: lab, Subject: "agente-1",
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}).SignedString(key)
	if err != nil {
		t.Fatalf("assinar: %v", err)
	}
	return signed
}

// #region mcphttp-deputy-test
// O servidor de terceiros nunca vê o token do agente, e a Guarita não
// aceita, como se fosse dela, um token emitido para esse servidor.
func TestGuarita_naoEhUmDeputadoConfuso(t *testing.T) {
	seen := make(chan string, 3)
	upstream := httptest.NewServer(authorizations(seen))
	t.Cleanup(upstream.Close)
	public, private, _ := ed25519.GenerateKey(nil)
	resource := deputy(t, upstream.URL, public)
	forServer := upstream.URL + "/mcp"

	for _, c := range []struct {
		name, bearer string
		status       int
	}{
		{"para a Guarita", token(t, private, resource), 200},
		{"para o servidor", token(t, private, forServer), 401},
		{"sem token", "", 401},
	} {
		status := post(t, resource, c.bearer)
		t.Logf("%-16s -> %d", c.name, status)
		if status != c.status {
			t.Errorf("%s: %d; want %d", c.name, status, c.status)
		}
	}
	got := <-seen
	if got != "Bearer cred-da-guarita" || len(seen) > 0 {
		t.Fatalf("o servidor recebeu %q e mais %d", got, len(seen))
	}
}

// #endregion

// authorizations registra o Authorization de cada pedido que chega.
func authorizations(seen chan<- string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		seen <- r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}
}

func post(t *testing.T, endpoint, bearer string) int {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
		`"params":{"name":"read_file"}}`
	r, _ := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}
