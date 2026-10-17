// Package stripauth mostra o repasse de credenciais do Capítulo 8: o
// proxy do Capítulo 7, que repassa tudo, e a correção ingênua, que apaga
// só o cabeçalho Authorization.
package stripauth

import (
	"net/http/httputil"
	"net/url"
)

// #region examples-stripauth-proxy
// Unchanged é o repasse do Capítulo 7: todos os cabeçalhos seguem.
func Unchanged(target *url.URL) *httputil.ReverseProxy {
	rewrite := func(r *httputil.ProxyRequest) { r.SetURL(target) }
	return &httputil.ReverseProxy{Rewrite: rewrite}
}

// StripAuthorization apaga o Authorization e repassa o resto.
func StripAuthorization(target *url.URL) *httputil.ReverseProxy {
	rewrite := func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Header.Del("Authorization")
	}
	return &httputil.ReverseProxy{Rewrite: rewrite}
}

// #endregion
