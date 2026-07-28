package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
)

type ProgramHandler struct {
	programs *repository.ProgramRepo
	ai       *service.AIService
}

func NewProgramHandler(programs *repository.ProgramRepo, ai *service.AIService) *ProgramHandler {
	return &ProgramHandler{programs: programs, ai: ai}
}

func (h *ProgramHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	templates, err := h.programs.GetTemplates(r.Context())
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, templates)
}

func (h *ProgramHandler) GetProgram(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	program, err := h.programs.GetActiveProgram(r.Context(), userID)
	if err != nil {
		jsonError(w, "no active program", http.StatusNotFound)
		return
	}

	workouts, err := h.programs.GetWorkoutsForProgram(r.Context(), program.ID)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}

	// build schedule (current week)
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	schedule := make([]map[string]any, 7)
	for i, day := range days {
		schedule[i] = map[string]any{"day": day, "status": "rest", "name": "Rest", "focus": "Active recovery"}
	}
	todayDow := int(time.Now().Weekday()+6) % 7
	for _, w := range workouts {
		if w.WeekNumber != program.CurrentWeek {
			continue
		}
		dow := w.DayOfWeek
		var status string
		if dow < todayDow {
			status = "done"
		} else if dow == todayDow {
			status = "today"
		} else {
			status = "upcoming"
		}
		schedule[dow] = map[string]any{
			"day":    days[dow],
			"name":   w.Name,
			"focus":  w.Focus,
			"status": status,
		}
	}

	jsonOK(w, map[string]any{
		"id":          program.ID,
		"name":        program.Name,
		"goal":        program.Goal,
		"level":       program.Level,
		"week":        program.CurrentWeek,
		"totalWeeks":  program.TotalWeeks,
		"daysPerWeek": program.DaysPerWeek,
		"schedule":    schedule,
	})
}

func (h *ProgramHandler) SetProgram(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		TemplateID string `json:"templateId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TemplateID == "" {
		jsonError(w, "templateId required", http.StatusBadRequest)
		return
	}

	templates, err := h.programs.GetTemplates(r.Context())
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	var found *model.PlanTemplate
	for i := range templates {
		if templates[i].ID == body.TemplateID {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		jsonError(w, "template not found", http.StatusNotFound)
		return
	}

	// Crea il programma e ci copia dentro i workout della template.
	programID, err := h.programs.CreateFromTemplate(r.Context(), userID, *found)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true, "programId": programID})
}

func (h *ProgramHandler) GeneratePlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var input service.GeneratePlanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if input.Goal == "" || input.Level == "" || input.Days == 0 {
		jsonError(w, "goal, level, days required", http.StatusBadRequest)
		return
	}
	if input.Length == 0 {
		input.Length = 60
	}

	program, err := h.ai.GeneratePlan(r.Context(), userID, input)
	if err != nil {
		jsonError(w, "AI generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, program)
}
