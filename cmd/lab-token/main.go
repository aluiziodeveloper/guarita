// Comando lab-token faz o papel do servidor de autorização no
// laboratório do Capítulo 8: gera o par de chaves Ed25519 do emissor e
// emite tokens JWT para o agente. Não é para produção.
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var errUsage = errors.New("uso: lab-token -genkey -out dir | " +
	"lab-token -key arquivo -iss emissor -aud recurso -sub agente")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatalf("lab-token: %v", err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("lab-token", flag.ContinueOnError)
	genkey := flags.Bool("genkey", false, "gera agent.key e agent.pub")
	out := flags.String("out", ".", "diretório das chaves geradas")
	keyFile := flags.String("key", "", "chave privada do emissor (PEM)")
	issuer := flags.String("iss", "", "emissor")
	audience := flags.String("aud", "", "recurso a que o token se destina")
	subject := flags.String("sub", "", "agente")
	ttl := flags.Duration("ttl", 15*time.Minute, "validade")
	if err := flags.Parse(args); err != nil {
		return errors.Join(errUsage, err)
	}
	if *genkey {
		return generate(*out)
	}
	if *keyFile == "" || *issuer == "" || *audience == "" || *subject == "" {
		return errUsage
	}
	token, err := mint(*keyFile, *issuer, *audience, *subject, *ttl)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, token)
	return err
}

// #region lab-token-generate
// generate grava a chave privada em PKCS #8 e a pública em PKIX, ambas
// em PEM. Só a privada é secreta, e o arquivo dela nasce com 0600.
func generate(dir string) error {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}
	return errors.Join(
		writePEM(filepath.Join(dir, "agent.key"), "PRIVATE KEY",
			privateDER, 0o600),
		writePEM(filepath.Join(dir, "agent.pub"), "PUBLIC KEY",
			publicDER, 0o644))
}

// #endregion

func writePEM(path, kind string, der []byte, mode os.FileMode) error {
	block := pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
	return os.WriteFile(path, block, mode)
}

// mint assina um token EdDSA com emissor, sujeito, audiência e validade.
func mint(
	keyFile, issuer, audience, subject string, ttl time.Duration,
) (string, error) {
	//nolint:gosec // G304: o caminho vem de quem roda o laboratório
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return "", err
	}
	key, err := jwt.ParseEdPrivateKeyFromPEM(data)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer: issuer, Subject: subject,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).
		SignedString(key)
}
