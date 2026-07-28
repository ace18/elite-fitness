package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// tooManyRequests risponde 429 con Retry-After, così il client sa quando ha
// senso riprovare invece di insistere.
func tooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	jsonError(w, "too many requests — try again later", http.StatusTooManyRequests)
}

// clientIP è la chiave del rate limit per IP. Usa r.RemoteAddr e basta: se il
// backend sta dietro un proxy, è quel proxy a dover riscrivere RemoteAddr
// (TRUST_PROXY_HEADERS abilita middleware.RealIP in main.go). Leggere qui
// X-Forwarded-For senza saperlo renderebbe il limite aggirabile con un header
// falso.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// validEmail — sbarramento minimo per non spendere una chiamata al provider su
// un indirizzo palesemente inservibile. Il frontend valida già, questo è il
// controllo lato server.
func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	// addr.Address != s scarta le forme tipo `Nome <a@b.c>`: qui vogliamo il
	// solo indirizzo.
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	return at > 0 && strings.Contains(s[at+1:], ".")
}
