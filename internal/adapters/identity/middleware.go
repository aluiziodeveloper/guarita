package identity

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// Middleware embrulha um handler, como Guard.Wrap no Capítulo 7.
type Middleware = func(http.Handler) http.Handler

// MetadataPath é onde o cliente descobre como obter um token para a
// Guarita (RFC 9728).
const MetadataPath = "/.well-known/oauth-protected-resource"

// #region identity-require
// Require recusa com 401 o pedido sem token válido e aponta, em
// WWW-Authenticate, os metadados do recurso protegido.
func Require(v *Verifier, metadataURL string) Middleware {
	options := &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
	}
	return auth.RequireBearerToken(v.Verify, options)
}

// Subject devolve o sujeito do token que Require aceitou. O SDK guarda
// o TokenInfo no contexto do pedido, com uma chave não exportada.
func Subject(ctx context.Context) string {
	if info := auth.TokenInfoFromContext(ctx); info != nil {
		return info.UserID
	}
	return ""
}

// #endregion

// Metadata serve os metadados do recurso: quem ele é e quem emite os
// tokens que ele aceita.
func Metadata(resource, issuer string) http.Handler {
	return auth.ProtectedResourceMetadataHandler(
		&oauthex.ProtectedResourceMetadata{
			Resource:               resource,
			AuthorizationServers:   []string{issuer},
			BearerMethodsSupported: []string{"header"},
		})
}
