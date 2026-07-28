package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/ratelimit"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
)

// Limiti di /api/auth/magic-link. L'endpoint è pubblico e ogni chiamata manda
// una email vera a un indirizzo scelto da chi chiama: senza freno è un modo
// gratuito per riempire la casella di qualcun altro e far salire il conto di
// Resend.
const (
	// Per IP: la difesa che conta davvero, perché è l'unica che ferma chi
	// cambia indirizzo a ogni richiesta. Larga abbastanza da non dare fastidio
	// a più persone dietro la stessa NAT.
	magicLinkPerIP    = 15
	magicLinkIPWindow = time.Hour
	// Per indirizzo: evita di ripetere l'invio allo stesso destinatario. Tenuta
	// corta di proposito — bloccare un indirizzo è anche un modo per impedire
	// al legittimo proprietario di entrare, quindi l'attesa deve essere breve.
	magicLinkPerEmail    = 3
	magicLinkEmailWindow = 15 * time.Minute
)

type AuthHandler struct {
	auth    *service.AuthService
	users   *repository.UserRepo
	byIP    *ratelimit.Limiter
	byEmail *ratelimit.Limiter
}

func NewAuthHandler(auth *service.AuthService, users *repository.UserRepo) *AuthHandler {
	return &AuthHandler{
		auth:    auth,
		users:   users,
		byIP:    ratelimit.New(magicLinkPerIP, magicLinkIPWindow),
		byEmail: ratelimit.New(magicLinkPerEmail, magicLinkEmailWindow),
	}
}

func (h *AuthHandler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		jsonError(w, "email required", http.StatusBadRequest)
		return
	}

	// Normalizzata prima di tutto: è la chiave del rate limit per indirizzo, e
	// senza questo "A@b.com" e "a@b.com" sarebbero due secchielli diversi.
	email := service.NormalizeEmail(body.Email)
	if !validEmail(email) {
		// Fermarlo qui evita una chiamata a Resend destinata a fallire.
		jsonError(w, "invalid email address", http.StatusBadRequest)
		return
	}

	// L'IP per primo: è il limite che ferma l'invio massivo verso indirizzi
	// sempre diversi, cioè l'abuso che costa di più.
	if !h.byIP.Allow(clientIP(r)) {
		tooManyRequests(w, h.byIP.RetryAfter(clientIP(r)))
		return
	}
	if !h.byEmail.Allow(email) {
		tooManyRequests(w, h.byEmail.RetryAfter(email))
		return
	}

	token, err := h.auth.SendMagicLink(r.Context(), email)
	if err != nil {
		// Il motivo (chiave Resend rifiutata, dominio non verificato, …) resta
		// nei log: al client basta sapere che non è partita.
		log.Printf("magic-link for %s: %v", email, err)
		jsonError(w, "failed to send link", http.StatusInternalServerError)
		return
	}
	res := map[string]any{"ok": true, "email": email}
	// SOLO in sviluppo: senza servizio email il token non arriverebbe da
	// nessuna parte, quindi lo restituiamo per permettere il login end-to-end.
	// In produzione (APP_ENV != development) NON viene mai incluso.
	if h.auth.IsDev() {
		res["devToken"] = token
	}
	jsonOK(w, res)
}

func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		jsonError(w, "token required", http.StatusBadRequest)
		return
	}
	user, jwt, err := h.auth.VerifyToken(r.Context(), token)
	if err != nil {
		jsonError(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
	jsonOK(w, map[string]any{"ok": true, "token": jwt, "user": user})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	jsonOK(w, user)
}
