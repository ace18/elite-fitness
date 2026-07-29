package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type ProgramHandler struct {
	programs *repository.ProgramRepo
	ai       *service.AIService
	planJobs *service.PlanJobStore
}

func NewProgramHandler(programs *repository.ProgramRepo, ai *service.AIService, planJobs *service.PlanJobStore) *ProgramHandler {
	return &ProgramHandler{programs: programs, ai: ai, planJobs: planJobs}
}

func (h *ProgramHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	templates, err := h.programs.GetTemplates(r.Context())
	if err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
		return
	}
	jsonOK(w, templates)
}

func (h *ProgramHandler) GetProgram(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	program, err := h.programs.GetActiveProgram(r.Context(), userID)
	if err != nil {
		jsonError(w, ErrNoActiveProgram, http.StatusNotFound)
		return
	}

	workouts, err := h.programs.GetWorkoutsForWeek(r.Context(), program.ID, program.CurrentWeek)
	if err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
		return
	}

	// Il calendario porta l'indice del giorno (0 = lunedì), non il nome: il
	// nome dipende dalla lingua dell'utente, che il backend non conosce. I
	// giorni di riposo hanno name/focus nulli e il client ci mette la sua copy.
	schedule := make([]map[string]any, 7)
	for i := range schedule {
		schedule[i] = map[string]any{"dayIndex": i, "status": "rest", "name": nil, "focus": nil}
	}
	todayDow := int(time.Now().Weekday()+6) % 7
	// Nessun filtro sulla settimana: ci ha già pensato la query. Filtrare qui
	// su week_number == CurrentWeek era il bug — dalla settimana 2 in poi non
	// corrispondeva più niente e il calendario si svuotava.
	for _, w := range workouts {
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
			"dayIndex": dow,
			"name":     w.Name,
			"focus":    w.Focus,
			"status":   status,
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
		jsonError(w, ErrTemplateRequired, http.StatusBadRequest)
		return
	}

	templates, err := h.programs.GetTemplates(r.Context())
	if err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
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
		jsonError(w, ErrTemplateNotFound, http.StatusNotFound)
		return
	}

	// Crea il programma e ci copia dentro i workout della template.
	programID, err := h.programs.CreateFromTemplate(r.Context(), userID, *found)
	if err != nil {
		jsonError(w, ErrDB, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"ok": true, "programId": programID})
}

func (h *ProgramHandler) GeneratePlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var input service.GeneratePlanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, ErrInvalidBody, http.StatusBadRequest)
		return
	}
	if input.Goal == "" || input.Level == "" || input.Days == 0 {
		jsonError(w, ErrPlanInput, http.StatusBadRequest)
		return
	}
	if input.Length == 0 {
		input.Length = 60
	}

	// Non si genera dentro la richiesta: ci vogliono minuti e il tunnel
	// Cloudflare stacca l'origine dopo ~100s. Si avvia il lavoro e si risponde
	// subito con l'id da interrogare (vedi service/planjob.go).
	job := h.planJobs.Start(userID,
		func(ctx context.Context) (*model.UserProgram, error) {
			return h.ai.GeneratePlan(ctx, userID, input)
		},
		func(err error) string {
			// Il dettaglio (chiave API, rifiuto del modello, …) resta nei log:
			// al client basta sapere che non è riuscita.
			log.Printf("generate plan for %s: %v", userID, err)
			return ErrPlanGeneration
		},
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{"jobId": job.ID})
}

// GetPlanJob — stato di una generazione avviata con POST /api/plans/generate.
func (h *ProgramHandler) GetPlanJob(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	job, ok := h.planJobs.Get(chi.URLParam(r, "jobId"), userID)
	if !ok {
		// Anche il job di un altro utente finisce qui: l'id da solo non deve
		// rivelare nulla, nemmeno che esiste.
		jsonError(w, ErrPlanJobNotFound, http.StatusNotFound)
		return
	}

	out := map[string]any{"status": string(job.Status)}
	switch job.Status {
	case service.PlanJobDone:
		out["program"] = job.Program
	case service.PlanJobFailed:
		out["code"] = job.ErrCode
		out["error"] = errorMessages[job.ErrCode]
	}
	jsonOK(w, out)
}
