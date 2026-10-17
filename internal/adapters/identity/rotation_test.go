package identity

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Desafio do capítulo 8: para trocar a chave do emissor sem derrubar os
// agentes, a Guarita aceita mais de uma chave, escolhida pelo kid do
// cabeçalho do JWT. Kid desconhecido, ou ausente com mais de uma chave,
// é recusado.
func TestVerifier_rotacaoPorKid(t *testing.T) {
	public1, private1, _ := ed25519.GenerateKey(nil)
	public2, private2, _ := ed25519.GenerateKey(nil)
	v, err := NewVerifierWithKeys(map[string]ed25519.PublicKey{
		"k1": public1, "k2": public2,
	}, issuer, resource)
	if err != nil {
		t.Fatalf("NewVerifierWithKeys() = %v", err)
	}
	v.now = func() time.Time { return now }
	withKid := func(kid string, key ed25519.PrivateKey) string {
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA,
			claimsFor(resource))
		if kid != "" {
			token.Header["kid"] = kid
		}
		signed, _ := token.SignedString(key)
		return signed
	}
	for _, c := range []struct {
		name  string
		token string
		valid bool
	}{
		{"k1", withKid("k1", private1), true},
		{"k2", withKid("k2", private2), true},
		{"k1 assinado com k2", withKid("k1", private2), false},
		{"kid desconhecido", withKid("k3", private1), false},
		{"sem kid", withKid("", private1), false},
	} {
		_, err := v.Verify(t.Context(), c.token, nil)
		if (err == nil) != c.valid {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}
