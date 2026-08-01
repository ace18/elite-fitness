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

// Codici d'errore stabili. Il client li traduce nella lingua dell'utente: il
// backend non sa e non deve sapere in che lingua sta guardando l'app.
const (
	ErrEmailRequired    = "email_required"
	ErrInvalidEmail     = "invalid_email"
	ErrRateLimited      = "rate_limited"
	ErrSendLinkFailed   = "send_link_failed"
	ErrTokenRequired    = "token_required"
	ErrInvalidToken     = "invalid_token"
	ErrUserNotFound     = "user_not_found"
	ErrUnknownProvider  = "unknown_provider"
	ErrDB               = "db_error"
	ErrNoActiveProgram  = "no_active_program"
	ErrTemplateRequired = "template_required"
	ErrTemplateNotFound = "template_not_found"
	ErrInvalidBody      = "invalid_body"
	ErrSaveSession      = "save_session_failed"
	ErrNoSessions       = "no_sessions"
	ErrNoWorkoutToday   = "no_workout_today"
	ErrPlanInput        = "plan_input_required"
	ErrWeightRequired   = "weight_required"
	ErrPlanGeneration   = "plan_generation_failed"
	ErrPlanJobNotFound  = "plan_job_not_found"
	ErrNotFound         = "not_found"
	ErrUnknownExercise  = "unknown_exercise"
)

// Testo inglese associato a ogni codice. Resta nella risposta come `error`:
// serve nei log e come ripiego se il client non conosce il codice.
var errorMessages = map[string]string{
	ErrEmailRequired:    "email required",
	ErrInvalidEmail:     "invalid email address",
	ErrRateLimited:      "too many requests — try again later",
	ErrSendLinkFailed:   "failed to send link",
	ErrTokenRequired:    "token required",
	ErrInvalidToken:     "invalid or expired token",
	ErrUserNotFound:     "user not found",
	ErrUnknownProvider:  "unknown provider",
	ErrDB:               "db error",
	ErrNoActiveProgram:  "no active program",
	ErrTemplateRequired: "templateId required",
	ErrTemplateNotFound: "template not found",
	ErrInvalidBody:      "invalid body",
	ErrSaveSession:      "failed to save session",
	ErrNoSessions:       "no sessions yet",
	ErrNoWorkoutToday:   "no workout today",
	ErrPlanInput:        "goal, level, days required",
	ErrWeightRequired:   "weight required",
	ErrPlanGeneration:   "plan generation failed",
	ErrPlanJobNotFound:  "plan generation not found",
	ErrNotFound:         "not found",
	ErrUnknownExercise:  "session references an exercise this server does not know",
}

func jsonError(w http.ResponseWriter, code string, status int) {
	msg, ok := errorMessages[code]
	if !ok {
		msg = code
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

// tooManyRequests risponde 429 con Retry-After, così il client sa quando ha
// senso riprovare invece di insistere.
func tooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	jsonError(w, ErrRateLimited, http.StatusTooManyRequests)
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
