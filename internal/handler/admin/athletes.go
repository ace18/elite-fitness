package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

// sessionListLimit — quanti allenamenti mostrare nella scheda atleta. Abbastanza
// da coprire un paio di mesi di allenamenti seri senza impaginare.
const sessionListLimit = 40

// staleDays — dopo quanti giorni senza allenarsi un atleta viene segnalato.
// Dieci: una settimana saltata capita a tutti, due sono un abbandono.
const staleDays = 10

type rosterData struct {
	pageData
	Athletes []model.AthleteRow
	// Contatori dell'intestazione, calcolati qui e non nel template: fare i
	// conti in un template significa scriverli in un linguaggio che non sa
	// contare.
	Total  int
	Active int
	Stale  int
}

func (h *Handler) roster(w http.ResponseWriter, r *http.Request) {
	athletes, err := h.admins.ListAthletes(r.Context())
	if err != nil {
		h.fail(w, r, err, "elenco atleti")
		return
	}

	data := rosterData{
		pageData: h.base(r, "Atleti", "atleti"),
		Athletes: athletes,
		Total:    len(athletes),
	}
	for i := range athletes {
		a := &athletes[i]
		if a.Sessions7d > 0 {
			data.Active++
		}
		if a.Stale(staleDays) {
			data.Stale++
		}
	}
	h.rn.render(w, http.StatusOK, "roster", data)
}

type athleteData struct {
	pageData
	Athlete  *model.AthleteRow
	Program  *model.UserProgram
	Metrics  *model.ProgressMetrics
	Sessions []model.SessionLog
	// Templates popola il menù di assegnazione.
	Templates []model.PlanTemplate
	// ProgressPct — quanto è avanzato il programma, per la barra. Calcolato qui
	// perché un template non sa dividere.
	ProgressPct int
	StaleDays   int
}

func (h *Handler) athlete(w http.ResponseWriter, r *http.Request) {
	h.renderAthlete(w, r, http.StatusOK, esitoMessage(r.URL.Query().Get("esito")), "")
}

func (h *Handler) renderAthlete(w http.ResponseWriter, r *http.Request, status int, flash, problem string) {
	id := chi.URLParam(r, "id")

	athlete, err := h.admins.FindAthlete(r.Context(), id)
	if err != nil {
		h.notFound(w, r, "Questo atleta non esiste.")
		return
	}

	data := athleteData{
		pageData:  h.base(r, athlete.Display(), "atleti"),
		Athlete:   athlete,
		StaleDays: staleDays,
	}
	data.Flash, data.Problem = flash, problem

	if templates, err := h.programs.GetTemplates(r.Context()); err == nil {
		data.Templates = templates
	} else {
		// Senza catalogo la scheda si vede lo stesso, manca solo il menù di
		// assegnazione. Non vale una pagina d'errore.
		h.logf("catalogo template: %v", err)
	}

	// Programma e statistiche mancano per chi si è iscritto e non ha ancora
	// fatto niente, che è uno stato normale: la pagina si disegna lo stesso e i
	// riquadri relativi non compaiono.
	if p, err := h.programs.GetActiveProgram(r.Context(), id); err == nil {
		data.Program = p
		if p.TotalWeeks > 0 {
			data.ProgressPct = p.CurrentWeek * 100 / p.TotalWeeks
		}
	}

	// weekGoal è l'obiettivo settimanale su cui GetProgressMetrics misura
	// l'aderenza. Senza programma non c'è un obiettivo da confrontare, e zero è
	// il modo di dirlo che il calcolo già gestisce.
	weekGoal := 0
	if data.Program != nil {
		weekGoal = data.Program.DaysPerWeek
	}
	if m, err := h.sessions.GetProgressMetrics(r.Context(), id, weekGoal); err == nil {
		data.Metrics = m
	} else {
		// Non è fatale — il resto della scheda è comunque utile — ma va saputo.
		h.logf("statistiche atleta %s: %v", id, err)
	}

	sessions, err := h.admins.ListAthleteSessions(r.Context(), id, sessionListLimit)
	if err != nil {
		h.fail(w, r, err, "storico allenamenti")
		return
	}
	data.Sessions = sessions

	h.rn.render(w, status, "athlete", data)
}

// assignProgram assegna un programma all'atleta, sostituendo quello attivo.
//
// La sostituzione la fa già CreateFromTemplate, che disattiva i programmi
// esistenti e crea il nuovo in un'unica transazione. Importante: li disattiva
// senza segnare completed_at, che è ciò che distingue "sostituito" da "portato
// a termine" — un programma interrotto a metà non deve risultare completato
// nello storico dell'atleta.
//
// Quello che l'atleta perde è la posizione nel programma precedente: il nuovo
// parte dalla settimana uno. È il motivo per cui il form dice a chiare lettere
// cosa sta sostituendo invece di limitarsi a un menù.
func (h *Handler) assignProgram(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	id := chi.URLParam(r, "id")

	// L'atleta va verificato prima di toccare qualsiasi cosa: senza, un id
	// inesistente creerebbe un programma appeso a nessuno.
	if _, err := h.admins.FindAthlete(r.Context(), id); err != nil {
		h.notFound(w, r, "Questo atleta non esiste.")
		return
	}

	templates, err := h.programs.GetTemplates(r.Context())
	if err != nil {
		h.fail(w, r, err, "catalogo template")
		return
	}
	templateID := r.PostFormValue("templateId")
	var chosen *model.PlanTemplate
	for i := range templates {
		if templates[i].ID == templateID {
			chosen = &templates[i]
			break
		}
	}
	if chosen == nil {
		h.renderAthlete(w, r, http.StatusBadRequest, "", "Scegli un programma dall'elenco.")
		return
	}

	oneRM, ok := parseKg(r.PostFormValue("oneRm"))
	if !ok {
		h.renderAthlete(w, r, http.StatusBadRequest, "",
			"Il massimale non è un numero valido. Usa per esempio 162,5.")
		return
	}
	// Il massimale serve a due condizioni diverse, e servono entrambe:
	// una finestra dichiarata (il programma vale solo in quel intervallo) e una
	// qualunque riga che calcoli il carico dal massimale. Controllando solo la
	// prima, una template scritta nel pannello con una percentuale ma senza
	// finestra passerebbe senza massimale, e CreateFromTemplate lascerebbe tutti
	// i carichi a NULL: la scheda dell'atleta arriverebbe senza pesi, senza che
	// nessuno abbia visto un errore.
	if (chosen.MinOneRM != nil || chosen.Prescribes) && oneRM == 0 {
		h.renderAthlete(w, r, http.StatusBadRequest, "",
			"«"+chosen.Name+"» calcola i carichi dal massimale: inserisci quello dell'atleta.")
		return
	}

	me, _ := r.Context().Value(adminCtxKey).(*model.Admin)
	if _, err := h.programs.AssignFromTemplate(r.Context(), id, *chosen, oneRM, me); err != nil {
		if errors.Is(err, repository.ErrOneRMOutOfRange) {
			h.renderAthlete(w, r, http.StatusBadRequest, "", outOfRangeMessage(chosen))
			return
		}
		h.fail(w, r, err, "assegnazione programma")
		return
	}

	http.Redirect(w, r, basePath+"/atleti/"+id+"?esito=programma-assegnato", http.StatusSeeOther)
}

// outOfRangeMessage spiega la finestra invece di dire solo che è sbagliata: chi
// assegna deve sapere quale massimale sarebbe andato bene, se no gli tocca
// indovinare.
func outOfRangeMessage(t *model.PlanTemplate) string {
	msg := "«" + t.Name + "» funziona solo per massimali "
	switch {
	case t.MinOneRM != nil && t.MaxOneRM != nil:
		msg += "fra " + decimal(*t.MinOneRM, 1) + " e " + decimal(*t.MaxOneRM, 1) + " kg."
	case t.MinOneRM != nil:
		msg += "da " + decimal(*t.MinOneRM, 1) + " kg in su."
	case t.MaxOneRM != nil:
		msg += "fino a " + decimal(*t.MaxOneRM, 1) + " kg."
	}
	return msg + " Gli incrementi sono in chili pieni e fuori da questa finestra le ultime settimane non stanno in piedi."
}

// parseKg legge un peso scritto da un italiano, che la virgola decimale la usa.
// Vuoto vale zero: i programmi che non prescrivono carichi non chiedono niente.
func parseKg(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

type sessionData struct {
	pageData
	Athlete *model.AthleteRow
	Session *model.SessionLog
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sessionID := chi.URLParam(r, "sessionID")

	athlete, err := h.admins.FindAthlete(r.Context(), id)
	if err != nil {
		h.notFound(w, r, "Questo atleta non esiste.")
		return
	}
	// GetAthleteSession filtra anche per utente: un id di sessione di un altro
	// atleta qui non produce una pagina con l'intestazione sbagliata, produce
	// un 404.
	s, err := h.admins.GetAthleteSession(r.Context(), id, sessionID)
	if err != nil {
		h.notFound(w, r, "Questo allenamento non esiste.")
		return
	}

	h.rn.render(w, http.StatusOK, "session", sessionData{
		pageData: h.base(r, s.Name, "atleti"),
		Athlete:  athlete,
		Session:  s,
	})
}
