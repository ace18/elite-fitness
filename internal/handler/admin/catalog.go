package admin

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

// ---- elenco template -------------------------------------------------------

type templatesData struct {
	pageData
	Templates []model.TemplateSummary
}

func (h *Handler) templates(w http.ResponseWriter, r *http.Request) {
	h.renderTemplates(w, r, http.StatusOK,
		esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) renderTemplates(w http.ResponseWriter, r *http.Request, status int, flash, problem string) {
	list, err := h.catalog.ListTemplates(r.Context())
	if err != nil {
		h.fail(w, r, err, "catalogo programmi")
		return
	}
	data := templatesData{pageData: h.base(r, "Programmi", "programmi"), Templates: list}
	data.Flash, data.Problem = flash, problem
	h.rn.render(w, status, "programs", data)
}

// slugPattern — cosa può essere un identificativo di template.
//
// Non è cosmetico: plan_templates.id è TEXT ed è la chiave che user_programs
// referenzia per sempre. Finisce anche negli URL del pannello. Minuscole,
// numeri e trattini tengono fuori spazi e accenti, che in un URL andrebbero
// codificati e renderebbero illeggibile ogni riferimento.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// slugify propone un identificativo a partire dal nome, così chi crea una
// template non deve inventarselo.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	in, problem := templateInputFrom(r)
	if problem != "" {
		h.renderTemplates(w, r, http.StatusBadRequest, "", problem)
		return
	}
	if in.ID == "" {
		in.ID = slugify(in.Name)
	}
	if !slugPattern.MatchString(in.ID) {
		h.renderTemplates(w, r, http.StatusBadRequest, "",
			"L'identificativo può contenere solo lettere minuscole, numeri e trattini (per esempio «forza-base»).")
		return
	}

	if err := h.catalog.CreateTemplate(r.Context(), in); err != nil {
		if errors.Is(err, repository.ErrTemplateExists) {
			h.renderTemplates(w, r, http.StatusConflict, "",
				"L'identificativo «"+in.ID+"» è già usato da un altro programma.")
			return
		}
		h.fail(w, r, err, "creazione template")
		return
	}
	// Dritti alla scheda: una template appena creata non ha giorni, e senza
	// giorni non serve a niente. Portarci subito è il seguito naturale.
	h.redirectWith(w, r, basePath+"/programmi/"+in.ID, "template-creata")
}

// templateInputFrom legge i campi comuni a creazione e modifica.
func templateInputFrom(r *http.Request) (repository.TemplateInput, string) {
	in := repository.TemplateInput{
		ID:          strings.TrimSpace(r.PostFormValue("id")),
		Name:        strings.TrimSpace(r.PostFormValue("name")),
		Goal:        strings.TrimSpace(r.PostFormValue("goal")),
		Focus:       strings.TrimSpace(r.PostFormValue("focus")),
		Level:       strings.TrimSpace(r.PostFormValue("level")),
		DaysPerWeek: atoiOr(r.PostFormValue("daysPerWeek"), 3),
		SessionMin:  atoiOr(r.PostFormValue("sessionMin"), 60),
		TotalWeeks:  atoiOr(r.PostFormValue("totalWeeks"), 8),
		Glyph:       strings.TrimSpace(r.PostFormValue("glyph")),
	}
	if in.Name == "" {
		return in, "Il nome è obbligatorio."
	}
	if in.Glyph == "" {
		in.Glyph = "💪"
	}
	if in.DaysPerWeek < 1 || in.DaysPerWeek > 7 {
		return in, "I giorni a settimana devono stare fra 1 e 7."
	}
	if in.TotalWeeks < 1 {
		return in, "La durata deve essere di almeno una settimana."
	}
	if tag := strings.TrimSpace(r.PostFormValue("tag")); tag != "" {
		in.Tag = &tag
	}

	// La finestra di massimali: o tutte e due o nessuna. Una sola metà darebbe
	// una template che accetta massimali senza limite da un lato, cioè proprio
	// il caso che gli incrementi in chili pieni non reggono.
	minRM, okMin := parseKg(r.PostFormValue("minOneRm"))
	maxRM, okMax := parseKg(r.PostFormValue("maxOneRm"))
	if !okMin || !okMax {
		return in, "I massimali devono essere numeri (per esempio 145 e 170)."
	}
	switch {
	case minRM > 0 && maxRM > 0:
		if minRM >= maxRM {
			return in, "Il massimale minimo deve essere più piccolo del massimo."
		}
		in.MinOneRM, in.MaxOneRM = &minRM, &maxRM
	case minRM > 0 || maxRM > 0:
		return in, "La finestra di massimali va indicata per intero: minimo e massimo, oppure nessuno dei due."
	}
	return in, ""
}

func atoiOr(s string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return v
}

// ---- scheda template -------------------------------------------------------

type templateData struct {
	pageData
	Template *model.TemplateSummary
	Blocks   []model.TemplateBlock
	// NextWeek — il numero di settimana proposto per un blocco nuovo.
	NextWeek int
}

func (h *Handler) templateDetail(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, http.StatusOK, esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) renderTemplate(w http.ResponseWriter, r *http.Request, status int, flash, problem string) {
	id := chi.URLParam(r, "id")
	t, err := h.catalog.FindTemplate(r.Context(), id)
	if err != nil {
		h.notFound(w, r, "Questo programma non esiste.")
		return
	}
	blocks, err := h.catalog.ListDays(r.Context(), id)
	if err != nil {
		h.fail(w, r, err, "giorni della template")
		return
	}

	data := templateData{
		pageData: h.base(r, t.Name, "programmi"),
		Template: t,
		Blocks:   blocks,
		NextWeek: 1,
	}
	if n := len(blocks); n > 0 {
		data.NextWeek = blocks[n-1].WeekNumber + 1
	}
	data.Flash, data.Problem = flash, problem
	h.rn.render(w, status, "program", data)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	in, problem := templateInputFrom(r)
	if problem != "" {
		h.renderTemplate(w, r, http.StatusBadRequest, "", problem)
		return
	}
	// L'identificativo arriva dall'URL, non dal form: è la chiave che i
	// programmi già creati referenziano, e cambiarla vorrebbe dire staccarli.
	in.ID = chi.URLParam(r, "id")

	if err := h.catalog.UpdateTemplate(r.Context(), in); err != nil {
		h.fail(w, r, err, "modifica template")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+in.ID, "template-modificata")
}

func (h *Handler) archiveTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	archive := r.PostFormValue("azione") == "archivia"

	if err := h.catalog.SetTemplateArchived(r.Context(), id, archive); err != nil {
		h.fail(w, r, err, "archiviazione template")
		return
	}
	if archive {
		h.redirectWith(w, r, basePath+"/programmi", "template-archiviata")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+id, "template-ripristinata")
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	t, err := h.catalog.FindTemplate(r.Context(), id)
	if err != nil {
		h.notFound(w, r, "Questo programma non esiste.")
		return
	}

	if err := h.catalog.DeleteTemplate(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrTemplateInUse) {
			h.renderTemplates(w, r, http.StatusConflict, "",
				"«"+t.Name+"» non si può eliminare: "+
					plural(t.InUse, "1 atleta l'ha", "%d atleti l'hanno")+
					" già ricevuto, e i loro programmi rimandano qui. Archivialo: sparisce da quelli assegnabili senza toccare chi lo sta seguendo.")
			return
		}
		h.fail(w, r, err, "cancellazione template")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi", "template-eliminata")
}

// ---- giorni ----------------------------------------------------------------

func (h *Handler) createDay(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		h.renderTemplate(w, r, http.StatusBadRequest, "", "Il nome del giorno è obbligatorio.")
		return
	}
	week := atoiOr(r.PostFormValue("weekNumber"), 1)
	dow := atoiOr(r.PostFormValue("dayOfWeek"), 0)
	if week < 1 || dow < 0 || dow > 6 {
		h.renderTemplate(w, r, http.StatusBadRequest, "", "Settimana o giorno non validi.")
		return
	}

	dayID, err := h.catalog.CreateDay(r.Context(), id, week, dow, name, r.PostFormValue("focus"))
	if err != nil {
		if errors.Is(err, repository.ErrDayExists) {
			h.renderTemplate(w, r, http.StatusConflict, "",
				"Nel blocco della settimana "+strconv.Itoa(week)+" il "+dayName(dow)+" è già occupato. Un giorno della settimana può comparire una volta sola per blocco.")
			return
		}
		h.fail(w, r, err, "creazione giorno")
		return
	}
	// Dritti al giorno appena creato: senza esercizi non serve a niente, e
	// aggiungerli è la cosa che si vuole fare subito dopo.
	h.redirectWith(w, r, basePath+"/programmi/"+id+"/giorni/"+dayID, "giorno-creato")
}

type dayData struct {
	pageData
	Template  *model.TemplateSummary
	Day       *model.TemplateDay
	Exercises []model.Exercise
	Editing   *model.TemplateExercise
}

func (h *Handler) dayDetail(w http.ResponseWriter, r *http.Request) {
	h.renderDay(w, r, http.StatusOK, r.URL.Query().Get("riga"),
		esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) renderDay(w http.ResponseWriter, r *http.Request, status int, editingRow, flash, problem string) {
	templateID := chi.URLParam(r, "id")
	dayID := chi.URLParam(r, "dayID")

	t, err := h.catalog.FindTemplate(r.Context(), templateID)
	if err != nil {
		h.notFound(w, r, "Questo programma non esiste.")
		return
	}
	day, err := h.catalog.FindDay(r.Context(), templateID, dayID)
	if err != nil {
		h.notFound(w, r, "Questo giorno non esiste.")
		return
	}
	library, err := h.catalog.ListExercises(r.Context())
	if err != nil {
		h.fail(w, r, err, "libreria esercizi")
		return
	}

	data := dayData{
		pageData:  h.base(r, day.Name, "programmi"),
		Template:  t,
		Day:       day,
		Exercises: library,
	}
	data.Flash, data.Problem = flash, problem
	for i := range day.Exercises {
		if day.Exercises[i].ID == editingRow {
			data.Editing = &day.Exercises[i]
			break
		}
	}
	h.rn.render(w, status, "day", data)
}

func (h *Handler) updateDay(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		h.renderDay(w, r, http.StatusBadRequest, "", "", "Il nome del giorno è obbligatorio.")
		return
	}
	week := atoiOr(r.PostFormValue("weekNumber"), 1)
	dow := atoiOr(r.PostFormValue("dayOfWeek"), 0)

	if err := h.catalog.UpdateDay(r.Context(), templateID, dayID, name,
		r.PostFormValue("focus"), week, dow); err != nil {
		if errors.Is(err, repository.ErrDayExists) {
			h.renderDay(w, r, http.StatusConflict, "", "",
				"Nel blocco della settimana "+strconv.Itoa(week)+" il "+dayName(dow)+" è già occupato da un altro giorno.")
			return
		}
		h.fail(w, r, err, "modifica giorno")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+templateID+"/giorni/"+dayID, "giorno-modificato")
}

func (h *Handler) deleteDay(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	if err := h.catalog.DeleteDay(r.Context(), templateID, dayID); err != nil {
		h.fail(w, r, err, "cancellazione giorno")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+templateID, "giorno-eliminato")
}

// ---- righe di esercizio ----------------------------------------------------

// exerciseRowFrom legge i campi di una riga di esercizio dal form.
func exerciseRowFrom(r *http.Request) (repository.ExerciseRowInput, string) {
	in := repository.ExerciseRowInput{
		ExerciseID:  r.PostFormValue("exerciseId"),
		Sets:        atoiOr(r.PostFormValue("sets"), 3),
		TargetReps:  atoiOr(r.PostFormValue("targetReps"), 8),
		RestSeconds: atoiOr(r.PostFormValue("restSeconds"), 90),
	}
	if in.ExerciseID == "" {
		return in, "Scegli un esercizio."
	}
	if in.Sets < 1 || in.TargetReps < 1 {
		return in, "Serie e ripetizioni devono essere almeno 1."
	}
	if in.RestSeconds < 0 {
		return in, "Il recupero non può essere negativo."
	}

	// Il carico prescritto è opzionale, ma la percentuale comanda: senza di
	// quella la materializzazione in CreateFromTemplate lascia NULL e lo scarto
	// in chili non verrebbe usato da nessuno. Meglio dirlo che ignorarlo in
	// silenzio.
	pct, okPct := parseKg(r.PostFormValue("loadPct"))
	off, okOff := parseKg(r.PostFormValue("loadOffsetKg"))
	if !okPct || !okOff {
		return in, "Percentuale e scarto devono essere numeri."
	}
	if pct > 0 {
		if pct > 200 {
			return in, "La percentuale del massimale sembra sbagliata: si scrive 65 per il 65%."
		}
		frac := pct / 100
		in.LoadPct = &frac
		if off != 0 {
			in.LoadOffsetKg = &off
		}
	} else if off != 0 {
		return in, "Lo scarto in chili vale solo insieme a una percentuale del massimale."
	}
	return in, ""
}

func (h *Handler) addExerciseRow(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	in, problem := exerciseRowFrom(r)
	if problem != "" {
		h.renderDay(w, r, http.StatusBadRequest, "", "", problem)
		return
	}
	if err := h.catalog.AddExerciseRow(r.Context(), dayID, in); err != nil {
		h.fail(w, r, err, "aggiunta esercizio")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+templateID+"/giorni/"+dayID, "riga-aggiunta")
}

func (h *Handler) updateExerciseRow(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	rowID := chi.URLParam(r, "rowID")
	in, problem := exerciseRowFrom(r)
	if problem != "" {
		h.renderDay(w, r, http.StatusBadRequest, rowID, "", problem)
		return
	}
	if err := h.catalog.UpdateExerciseRow(r.Context(), dayID, rowID, in); err != nil {
		h.fail(w, r, err, "modifica riga")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+templateID+"/giorni/"+dayID, "riga-modificata")
}

func (h *Handler) deleteExerciseRow(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	if err := h.catalog.DeleteExerciseRow(r.Context(), dayID, chi.URLParam(r, "rowID")); err != nil {
		h.fail(w, r, err, "cancellazione riga")
		return
	}
	h.redirectWith(w, r, basePath+"/programmi/"+templateID+"/giorni/"+dayID, "riga-eliminata")
}

func (h *Handler) moveExerciseRow(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	templateID, dayID := chi.URLParam(r, "id"), chi.URLParam(r, "dayID")
	up := r.PostFormValue("verso") == "su"
	if err := h.catalog.MoveExerciseRow(r.Context(), dayID, chi.URLParam(r, "rowID"), up); err != nil {
		h.fail(w, r, err, "spostamento riga")
		return
	}
	// Senza esito: spostare una riga si vede da sé, e un messaggio a ogni clic
	// su una freccia diventa rumore.
	h.redirectWith(w, r, basePath+"/programmi/"+templateID+"/giorni/"+dayID, "")
}
