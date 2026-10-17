package identity

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	issuer   = "https://auth.lab.test"
	resource = "http://127.0.0.1:8080/mcp"
)

var now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func claimsFor(audience string) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer: issuer, Subject: "agente-1",
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any,
	claims jwt.RegisteredClaims,
) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("assinar: %v", err)
	}
	return token
}

func newVerifier(t *testing.T) (*Verifier, ed25519.PrivateKey) {
	t.Helper()
	public, private, _ := ed25519.GenerateKey(nil)
	v, err := NewVerifier(public, issuer, resource)
	if err != nil {
		t.Fatalf("NewVerifier() = %v", err)
	}
	v.now = func() time.Time { return now }
	return v, private
}

func TestVerify_fechaCadaPorta(t *testing.T) {
	v, private := newVerifier(t)
	_, stranger, _ := ed25519.GenerateKey(nil)
	edDSA, ok := jwt.SigningMethodEdDSA, claimsFor(resource)
	expired, noSubject := ok, ok
	expired.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
	noSubject.Subject = ""
	// #region identity-verify-test
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{"válido", sign(t, edDSA, private, ok), true},
		{"outro recurso", sign(t, edDSA, private,
			claimsFor("https://files.lab.test/mcp")), false},
		{"expirado", sign(t, edDSA, private, expired), false},
		{"sem sujeito", sign(t, edDSA, private, noSubject), false},
		{"outra chave", sign(t, edDSA, stranger, ok), false},
		{"HS256 com a chave pública", sign(t, jwt.SigningMethodHS256,
			[]byte(v.key), ok), false},
		{"alg none", sign(t, jwt.SigningMethodNone,
			jwt.UnsafeAllowNoneSignatureType, ok), false},
	}
	for _, test := range tests {
		info, err := v.Verify(t.Context(), test.token, nil)
		t.Logf("%-26s -> %v", test.name, describe(err, info))
		if (err == nil) != test.valid {
			t.Errorf("%s: err = %v", test.name, err)
		}
	}
}

// #endregion

func describe(err error, info *auth.TokenInfo) any {
	if err != nil {
		return err
	}
	return "sujeito " + info.UserID
}

func TestVerify_erroEhInvalidToken(t *testing.T) {
	v, _ := newVerifier(t)
	_, err := v.Verify(t.Context(), "não é jwt", nil)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err = %v; want auth.ErrInvalidToken", err)
	}
}

func TestNewVerifier_exigeConfiguracao(t *testing.T) {
	public, _, _ := ed25519.GenerateKey(nil)
	for _, c := range [][2]string{{"", resource}, {issuer, ""}} {
		if _, err := NewVerifier(public, c[0], c[1]); !errors.Is(err,
			ErrConfig) {
			t.Errorf("NewVerifier(%q, %q) = %v", c[0], c[1], err)
		}
	}
}

// #region identity-require-test
// Sem token, 401 com o caminho dos metadados; com token, o handler de
// dentro recebe o sujeito no contexto.
func TestRequire_recusaSemTokenEPassaOSujeito(t *testing.T) {
	v, private := newVerifier(t)
	v.now = time.Now // o middleware do SDK confere exp no relógio real
	const metadata = "http://127.0.0.1:8080" + MetadataPath
	var subject string
	handler := Require(v, metadata)(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			subject = Subject(r.Context())
		}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("POST", "/", nil))
	challenge := recorder.Header().Get("WWW-Authenticate")
	t.Logf("sem token: %d, %s", recorder.Code, challenge)
	if recorder.Code != 401 || !strings.Contains(challenge, metadata) {
		t.Fatalf("sem token: %d %q", recorder.Code, challenge)
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Authorization", "Bearer "+liveToken(t, private))
	handler.ServeHTTP(httptest.NewRecorder(), r)
	if subject != "agente-1" {
		t.Fatalf("sujeito no contexto = %q", subject)
	}
}

// #endregion

// liveToken assina um token válido no relógio real.
func liveToken(t *testing.T, private ed25519.PrivateKey) string {
	t.Helper()
	claims := claimsFor(resource)
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute))
	return sign(t, jwt.SigningMethodEdDSA, private, claims)
}
