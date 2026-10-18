package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// #region cmd-guarita-defaults-test
// Os três padrões: sem token só com a opção escrita; fora do loopback,
// identidade e TLS. Nenhuma das três Guaritas chega a escutar.
func TestRun_padroesSeguros(t *testing.T) {
	for _, c := range []struct {
		args []string
		want error
	}{
		{[]string{}, errNoAuth},
		{[]string{"-host", "guarita.lab.test:8443"}, errAuthRequired},
		{[]string{
			"-host", "guarita.lab.test:8443",
			"-agent-key", "chave.pem",
		}, errTLSRequired},
	} {
		args := append(c.args, "-listen", "127.0.0.1:0",
			"-upstream", "http://127.0.0.1:1/mcp")
		err := run(t.Context(), args, io.NopCloser(nothing),
			nopWriteCloser{&bytes.Buffer{}}, io.Discard)
		t.Logf("%-44q %v", strings.Join(c.args, " "), err)
		if !errors.Is(err, c.want) {
			t.Errorf("%q: %v; quer %v", c.args, err, c.want)
		}
	}
}

// #endregion

// #region cmd-guarita-tls-test
// Com certificado, a Guarita fala TLS 1.3: o cliente que confia nele é
// atendido; o que só fala até o TLS 1.2 é recusado no aperto de mão.
func TestRun_tls(t *testing.T) {
	cert, key, pool := certificate(t)
	upstream := echoUpstream(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	endpoint, _ := startHTTP(t, ctx, "-allow", "read_file",
		"-tls-cert", cert, "-tls-key", key, "-listen", "127.0.0.1:0",
		"-upstream", upstream)
	versions := []uint16{tls.VersionTLS13, tls.VersionTLS12}
	for _, maxVersion := range versions {
		client := &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				MaxVersion: maxVersion,
			},
		}}
		res, err := client.Post(endpoint, "application/json",
			strings.NewReader(toolCall(1, "read_file")))
		if err == nil {
			_ = res.Body.Close()
		}
		t.Logf("%s: %v", tls.VersionName(maxVersion), status(res, err))
		if (maxVersion == tls.VersionTLS13) != (err == nil) {
			t.Errorf("%s: %v", tls.VersionName(maxVersion), err)
		}
	}
}

// #endregion

// nothing é a entrada padrão vazia do modo HTTP.
var nothing = strings.NewReader("")

func status(res *http.Response, err error) string {
	if err != nil {
		_, msg, _ := strings.Cut(err.Error(), ": remote error: ")
		return "recusado: " + msg
	}
	return res.Status
}

// echoUpstream é um servidor MCP de mentira, que responde a tudo.
func echoUpstream(t *testing.T) string {
	t.Helper()
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	server := &http.Server{Handler: http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
		}), ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })
	return "http://" + l.Addr().String() + "/mcp"
}

// certificate gera um certificado autoassinado para 127.0.0.1 e devolve
// os arquivos PEM e o conjunto de raízes que confia nele.
func certificate(t *testing.T) (cert, key string, pool *x509.CertPool) {
	t.Helper()
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "guarita.lab"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl,
		&priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(priv)
	dir := t.TempDir()
	cert, key = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	_ = os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: der,
	}), 0o600)
	_ = os.WriteFile(key, pem.EncodeToMemory(&pem.Block{
		Type: "PRIVATE KEY", Bytes: keyDER,
	}), 0o600)
	parsed, _ := x509.ParseCertificate(der)
	pool = x509.NewCertPool()
	pool.AddCert(parsed)
	return cert, key, pool
}
