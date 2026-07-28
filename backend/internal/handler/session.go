package handler

import (
	"encoding/json"
	"net/http"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
)

type SessionHandler struct {
	sessions *repository.SessionRepo
}

func NewSessionHandler(sessions *repository.SessionRepo) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

func (h *SessionHandler) SaveSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var s model.SessionLog
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.UserID = userID
	if err := h.sessions.SaveSession(r.Context(), &s); err != nil {
		jsonError(w, "failed to save session", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true, "id": s.ID})
}

func (h *SessionHandler) GetLastSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	s, err := h.sessions.GetLastSession(r.Context(), userID)
	if err != nil {
		jsonError(w, "no sessions yet", http.StatusNotFound)
		return
	}
	jsonOK(w, s)
}
