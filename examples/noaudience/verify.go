// Package noaudience é a verificação ingênua de JWT do Capítulo 8:
// confere assinatura, algoritmo, emissor e validade, mas não para quem o
// token foi emitido.
package noaudience

import (
	"crypto/ed25519"

	"github.com/golang-jwt/jwt/v5"
)

// #region examples-noaudience-verify
// Verify devolve o sujeito de um token assinado pela chave do emissor.
// Falta uma pergunta: o token foi emitido para a Guarita?
func Verify(token string, key ed25519.PublicKey, issuer string) (
	string, error,
) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired())
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// #endregion
