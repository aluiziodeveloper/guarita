package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"path/filepath"
	"time"
)

// #region lab-token-cert
// certificate grava um certificado autoassinado para host e para o
// loopback, válido por um dia: o bastante para o laboratório, e curto
// para não servir a mais nada. A chave nasce com 0600.
func certificate(dir, host string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tmpl := template(host)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl,
		&key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return errors.Join(
		writePEM(filepath.Join(dir, "tls.crt"), "CERTIFICATE", der,
			0o644),
		writePEM(filepath.Join(dir, "tls.key"), "PRIVATE KEY", keyDER,
			0o600))
}

// #endregion

// template é o certificado de servidor do laboratório.
func template(host string) *x509.Certificate {
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host, "localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}
