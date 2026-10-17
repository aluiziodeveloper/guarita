package noaudience

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// #region examples-noaudience-test
// O emissor é o mesmo, a assinatura é válida, mas o token foi pedido
// para outro servidor. A verificação ingênua o aceita mesmo assim.
func TestVerify_aceitaTokenDeOutroRecurso(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	const issuer = "https://auth.lab.test"
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "agente-1",
		Audience:  jwt.ClaimStrings{"https://files.lab.test/mcp"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).
		SignedString(private)
	if err != nil {
		t.Fatalf("assinar: %v", err)
	}

	subject, err := Verify(token, public, issuer)
	t.Logf("aud=%v aceito como %q, err=%v", claims.Audience, subject,
		err)
	if err != nil {
		t.Fatalf("Verify() = %v; a versão ingênua deveria aceitar", err)
	}
}

// #endregion
