// Package identity verifica quem chama a Guarita: tokens JWT do agente,
// assinados em Ed25519 pelo emissor configurado e emitidos para ela.
package identity

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"maps"
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
	// ErrUnknownKey recusa um kid que não está entre as chaves.
	ErrUnknownKey = errors.New("unknown key id")
)

// leeway tolera diferença de relógio entre o emissor e a Guarita.
const leeway = 30 * time.Second

// Verifier confere os tokens do agente.
type Verifier struct {
	key      ed25519.PublicKey            // chave única, sem kid
	keys     map[string]ed25519.PublicKey // chaves por kid, na rotação
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
	return &Verifier{
		key: key, issuer: issuer, audience: audience, now: time.Now,
	}, nil
}

// NewVerifierWithKeys aceita várias chaves do emissor, escolhidas pelo
// kid do token. Serve para trocar a chave sem derrubar os agentes.
func NewVerifierWithKeys(
	keys map[string]ed25519.PublicKey, issuer, audience string,
) (*Verifier, error) {
	if len(keys) == 0 || issuer == "" || audience == "" {
		return nil, ErrConfig
	}
	for _, key := range keys {
		if len(key) != ed25519.PublicKeySize {
			return nil, ErrConfig
		}
	}
	return &Verifier{
		keys: maps.Clone(keys), issuer: issuer, audience: audience,
		now: time.Now,
	}, nil
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
// WithValidMethods antes de a chave ser pedida. Com várias chaves, o kid
// escolhe; sem kid ou com kid desconhecido, não há chave.
func (v *Verifier) keyFunc(token *jwt.Token) (any, error) {
	if v.keys == nil {
		return v.key, nil
	}
	kid, _ := token.Header["kid"].(string)
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKey, kid)
	}
	return key, nil
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
