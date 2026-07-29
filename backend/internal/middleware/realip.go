package middleware

import (
	"net"
	"net/http"
)

// CloudflareIP riscrive RemoteAddr con l'IP che Cloudflare mette in
// CF-Connecting-IP, l'unico header attendibile dietro il tunnel.
//
// Non usare chi/middleware.RealIP qui: quello legge, in ordine,
// True-Client-IP, X-Real-IP e infine il PRIMO valore di X-Forwarded-For.
// Cloudflare però *accoda* l'IP reale a un X-Forwarded-For già presente, quindi
// una richiesta con `X-Forwarded-For: 1.2.3.4` arriva come `1.2.3.4, <ip vero>`
// e RealIP terrebbe 1.2.3.4. Stessa storia per True-Client-IP, che Cloudflare
// popola solo sui piani Enterprise e altrove lascia passare così com'è.
// Il risultato sarebbe esattamente il rate limiting aggirabile con un header
// falso che TRUST_PROXY_HEADERS dovrebbe impedire.
//
// CF-Connecting-IP invece Cloudflare lo riscrive sempre, scartando qualsiasi
// valore in arrivo dal client.
func CloudflareIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" && net.ParseIP(ip) != nil {
			// La porta non serve a nessuno qui, ma clientIP() fa SplitHostPort
			// e senza porta cadrebbe nel ramo d'errore.
			r.RemoteAddr = net.JoinHostPort(ip, "0")
		}
		// Se l'header non c'è la richiesta non è passata dal tunnel: si tiene
		// RemoteAddr, che è comunque il peer reale.
		next.ServeHTTP(w, r)
	})
}

// StripForwardedHeaders cancella gli header di forwarding in ingresso prima che
// li veda chiunque altro. Difesa in profondità: nulla nel backend li legge, ma
// così un domani nessuno può reintrodurre il bypass leggendoli per sbaglio.
func StripForwardedHeaders(next http.Handler) http.Handler {
	drop := []string{"X-Forwarded-For", "X-Real-IP", "True-Client-IP"}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range drop {
			r.Header.Del(h)
		}
		next.ServeHTTP(w, r)
	})
}
