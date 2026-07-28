package handler

import (
	"encoding/json"
	"net/http"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/repository"
)

type ProgressHandler struct {
	sessions *repository.SessionRepo
	programs *repository.ProgramRepo
}

func NewProgressHandler(sessions *repository.SessionRepo, programs *repository.ProgramRepo) *ProgressHandler {
	return &ProgressHandler{sessions: sessions, programs: programs}
}

func (h *ProgressHandler) GetProgress(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	weekGoal := 3
	if p, err := h.programs.GetActiveProgram(r.Context(), userID); err == nil {
		weekGoal = p.DaysPerWeek
	}
	metrics, err := h.sessions.GetProgressMetrics(r.Context(), userID, weekGoal)
	if err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
		return
	}
	jsonOK(w, metrics)
}

func (h *ProgressHandler) LogWeight(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Weight float64 `json:"weight"`
		Unit   string  `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Weight == 0 {
		jsonError(w, ErrWeightRequired, http.StatusBadRequest)
		return
	}
	if body.Unit == "" {
		body.Unit = "kg"
	}
	if err := h.sessions.LogBodyWeight(r.Context(), userID, body.Weight, body.Unit); err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true})
}
