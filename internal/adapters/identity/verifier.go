// Package identity verifica quem chama a Guarita: tokens JWT do agente,
// assinados em Ed25519 pelo emissor configurado e emitidos para ela.
package identity

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// Erros de configuração do verificador.
var (
	ErrConfig    = errors.New("issuer, audience and key are required")
	ErrPublicKey = errors.New("ed25519 public key in PEM expected")
	ErrNoSubject = errors.New("token has no subject")
)

// leeway tolera diferença de relógio entre o emissor e a Guarita.
const leeway = 30 * time.Second

// Verifier confere os tokens do agente.
type Verifier struct {
	key      ed25519.PublicKey
	issuer   string
	audience string
	now      func() time.Time
}

// #region identity-new-verifier
// NewVerifier exige a chave pública do emissor, o emissor e a
// audiência: o endereço do recurso que a Guarita protege.
func NewVerifier(
	key ed25519.PublicKey, issuer, audience string,
) (*Verifier, error) {
	if len(key) != ed25519.PublicKeySize || issuer == "" ||
		audience == "" {
		return nil, ErrConfig
	}
	return &Verifier{key, issuer, audience, time.Now}, nil
}

// #endregion

// #region identity-verify
// Verify tem a assinatura de auth.TokenVerifier, do SDK. Cada opção do
// parser fecha uma porta: algoritmo, emissor, audiência e validade.
func (v *Verifier) Verify(
	_ context.Context, token string, _ *http.Request,
) (*auth.TokenInfo, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, v.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
		jwt.WithTimeFunc(v.now))
	if err == nil && claims.Subject == "" {
		err = ErrNoSubject
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, err)
	}
	return &auth.TokenInfo{
		UserID: claims.Subject, Expiration: claims.ExpiresAt.Time,
	}, nil
}

// #endregion

// keyFunc devolve a chave do emissor. O algoritmo já foi conferido por
// WithValidMethods antes de a chave ser pedida.
func (v *Verifier) keyFunc(*jwt.Token) (any, error) {
	return v.key, nil
}

// ParsePublicKey lê uma chave pública Ed25519 em PEM (PKIX).
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	key, err := jwt.ParseEdPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPublicKey, err)
	}
	public, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, ErrPublicKey
	}
	return public, nil
}
