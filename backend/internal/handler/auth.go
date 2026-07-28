package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
)

type AuthHandler struct {
	auth  *service.AuthService
	users *repository.UserRepo
}

func NewAuthHandler(auth *service.AuthService, users *repository.UserRepo) *AuthHandler {
	return &AuthHandler{auth: auth, users: users}
}

func (h *AuthHandler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		jsonError(w, "email required", http.StatusBadRequest)
		return
	}
	token, err := h.auth.SendMagicLink(r.Context(), body.Email)
	if err != nil {
		// Il motivo (chiave Resend rifiutata, dominio non verificato, …) resta
		// nei log: al client basta sapere che non è partita.
		log.Printf("magic-link for %s: %v", body.Email, err)
		jsonError(w, "failed to send link", http.StatusInternalServerError)
		return
	}
	res := map[string]any{"ok": true, "email": body.Email}
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
