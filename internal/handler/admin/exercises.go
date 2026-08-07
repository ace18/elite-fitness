package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

type exercisesData struct {
	pageData
	Exercises []model.Exercise
	// Editing — l'esercizio aperto in modifica, nil quando si sta solo
	// guardando l'elenco. Il form di modifica è la stessa pagina con una riga
	// espansa: una pagina a sé per cambiare tre campi costringerebbe a un
	// avanti e indietro per ogni ritocco.
	Editing *model.Exercise
	Muscles []string
}

// muscoli — i gruppi già presenti in libreria, per il menù a tendina. Presi dai
// dati invece che da un elenco fisso: la libreria è modificabile, e un elenco
// scritto nel codice comincerebbe a divergere dal primo esercizio aggiunto con
// un gruppo nuovo. Il campo resta comunque libero.
func muscleGroups(exercises []model.Exercise) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range exercises {
		if e.MuscleGroup != "" && !seen[e.MuscleGroup] {
			seen[e.MuscleGroup] = true
			out = append(out, e.MuscleGroup)
		}
	}
	return out
}

func (h *Handler) exercises(w http.ResponseWriter, r *http.Request) {
	h.renderExercises(w, r, http.StatusOK, "",
		esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) editExercise(w http.ResponseWriter, r *http.Request) {
	h.renderExercises(w, r, http.StatusOK, chi.URLParam(r, "id"),
		esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) renderExercises(w http.ResponseWriter, r *http.Request, status int, editingID, flash, problem string) {
	list, err := h.catalog.ListExercises(r.Context())
	if err != nil {
		h.fail(w, r, err, "libreria esercizi")
		return
	}

	data := exercisesData{
		pageData:  h.base(r, "Esercizi", "esercizi"),
		Exercises: list,
		Muscles:   muscleGroups(list),
	}
	data.Flash, data.Problem = flash, problem
	for i := range list {
		if list[i].ID == editingID {
			data.Editing = &list[i]
			break
		}
	}
	h.rn.render(w, status, "exercises", data)
}

func (h *Handler) createExercise(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	muscle := strings.TrimSpace(r.PostFormValue("muscleGroup"))
	if name == "" || muscle == "" {
		h.renderExercises(w, r, http.StatusBadRequest, "", "",
			"Nome e gruppo muscolare sono obbligatori.")
		return
	}

	if _, err := h.catalog.CreateExercise(r.Context(), name, muscle, r.PostFormValue("category")); err != nil {
		if errors.Is(err, repository.ErrExerciseExists) {
			h.renderExercises(w, r, http.StatusConflict, "", "",
				"«"+name+"» è già in libreria. I nomi sono unici — è quello che evita di ritrovarsi lo stesso esercizio due volte con storici separati.")
			return
		}
		h.fail(w, r, err, "creazione esercizio")
		return
	}
	h.redirectWith(w, r, basePath+"/esercizi", "esercizio-creato")
}

func (h *Handler) updateExercise(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	name := strings.TrimSpace(r.PostFormValue("name"))
	muscle := strings.TrimSpace(r.PostFormValue("muscleGroup"))
	if name == "" || muscle == "" {
		h.renderExercises(w, r, http.StatusBadRequest, id, "",
			"Nome e gruppo muscolare sono obbligatori.")
		return
	}

	if err := h.catalog.UpdateExercise(r.Context(), id, name, muscle, r.PostFormValue("category")); err != nil {
		if errors.Is(err, repository.ErrExerciseExists) {
			h.renderExercises(w, r, http.StatusConflict, id, "",
				"C'è già un esercizio che si chiama «"+name+"».")
			return
		}
		h.fail(w, r, err, "modifica esercizio")
		return
	}
	h.redirectWith(w, r, basePath+"/esercizi", "esercizio-modificato")
}

func (h *Handler) deleteExercise(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")

	// Letto prima per poter spiegare *perché* non si può, invece di dire solo
	// che non si può: "è in 3 template" e "è in 120 serie registrate" portano a
	// due decisioni diverse.
	ex, err := h.catalog.FindExercise(r.Context(), id)
	if err != nil {
		h.notFound(w, r, "Questo esercizio non esiste.")
		return
	}

	if err := h.catalog.DeleteExercise(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrExerciseInUse) {
			h.renderExercises(w, r, http.StatusConflict, "", "", inUseMessage(ex))
			return
		}
		h.fail(w, r, err, "cancellazione esercizio")
		return
	}
	h.redirectWith(w, r, basePath+"/esercizi", "esercizio-eliminato")
}

func inUseMessage(e *model.Exercise) string {
	var parts []string
	if e.InTemplates > 0 {
		parts = append(parts, plural(e.InTemplates, "1 riga di template", "%d righe di template"))
	}
	if e.InHistory > 0 {
		parts = append(parts, plural(e.InHistory, "1 serie registrata", "%d serie registrate"))
	}
	msg := "«" + e.Name + "» non si può eliminare: è usato in " + strings.Join(parts, " e ") + "."
	if e.InHistory > 0 {
		msg += " Lo storico degli atleti non si tocca — se non ti serve più, toglilo dalle template e lascialo in libreria."
	} else {
		msg += " Toglilo prima da quelle template."
	}
	return msg
}
