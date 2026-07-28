package handler

import (
	"net/http"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/service"
)

type WorkoutHandler struct {
	workouts *service.WorkoutService
}

func NewWorkoutHandler(workouts *service.WorkoutService) *WorkoutHandler {
	return &WorkoutHandler{workouts: workouts}
}

func (h *WorkoutHandler) GetToday(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	workout, err := h.workouts.BuildTodayWorkout(r.Context(), userID)
	if err != nil {
		jsonError(w, "no active program", http.StatusNotFound)
		return
	}
	if workout == nil {
		jsonError(w, "no workout today", http.StatusNotFound)
		return
	}
	jsonOK(w, workout)
}
