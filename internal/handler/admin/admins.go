package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type adminsData struct {
	pageData
	Admins []model.Admin
}

func (h *Handler) adminsPage(w http.ResponseWriter, r *http.Request) {
	h.renderAdmins(w, r, http.StatusOK, "", "")
}

// renderAdmins ridisegna l'elenco con un eventuale esito in cima. L'elenco si
// rilegge sempre dal database invece di modificare quello già in mano: dopo una
// creazione o una disattivazione la pagina deve mostrare com'è adesso, non come
// pensiamo di averlo lasciato.
func (h *Handler) renderAdmins(w http.ResponseWriter, r *http.Request, status int, flash, problem string) {
	list, err := h.admins.List(r.Context())
	if err != nil {
		h.fail(w, r, err, "elenco amministratori")
		return
	}
	data := adminsData{pageData: h.base(r, "Amministratori", "amministratori"), Admins: list}
	data.Flash, data.Problem = flash, problem
	h.rn.render(w, status, "admins", data)
}

func (h *Handler) createAdmin(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	me, _ := r.Context().Value(adminCtxKey).(*model.Admin)
	email := service.NormalizeEmail(r.PostFormValue("email"))

	if !validEmail(email) {
		h.renderAdmins(w, r, http.StatusBadRequest, "", "Indirizzo email non valido.")
		return
	}

	if _, err := h.admins.Create(r.Context(), email, me.ID); err != nil {
		if errors.Is(err, repository.ErrAdminExists) {
			// Anche il caso "c'è ma è disattivato" finisce qui: la riga esiste,
			// quindi la strada è riattivarla dall'elenco, non crearne un'altra.
			h.renderAdmins(w, r, http.StatusConflict, "",
				email+" è già fra gli amministratori. Se non compare attivo, riattivalo dall'elenco.")
			return
		}
		h.fail(w, r, err, "creazione amministratore")
		return
	}

	// Nessuna email automatica: il nuovo amministratore entra chiedendo lui il
	// link dalla pagina di accesso. Mandargliene uno adesso significherebbe
	// spedire una sessione pronta all'uso a un indirizzo appena digitato — un
	// carattere sbagliato e il link è di qualcun altro.
	h.renderAdmins(w, r, http.StatusOK,
		email+" è stato aggiunto. Può entrare dalla pagina di accesso richiedendo il link.", "")
}

func (h *Handler) toggleAdmin(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	me, _ := r.Context().Value(adminCtxKey).(*model.Admin)
	id := chi.URLParam(r, "id")
	disable := r.PostFormValue("azione") == "disattiva"

	// Disattivarsi da soli si può fare per sbaglio con un clic, e l'effetto è
	// chiudersi fuori. Non è la protezione sull'ultimo amministratore — quella
	// sta nella query — è un'altra cosa: qui c'è ancora qualcun altro, ma non
	// necessariamente qualcuno che ti rifaccia entrare oggi.
	if disable && id == me.ID {
		h.renderAdmins(w, r, http.StatusBadRequest, "",
			"Non puoi disattivare te stesso. Chiedi a un altro amministratore.")
		return
	}

	if err := h.admins.SetDisabled(r.Context(), id, disable); err != nil {
		if errors.Is(err, repository.ErrLastAdmin) {
			h.renderAdmins(w, r, http.StatusBadRequest, "",
				"Non puoi disattivare l'ultimo amministratore attivo: nessuno potrebbe più entrare.")
			return
		}
		h.fail(w, r, err, "stato amministratore")
		return
	}

	if disable {
		h.renderAdmins(w, r, http.StatusOK, "Amministratore disattivato.", "")
		return
	}
	h.renderAdmins(w, r, http.StatusOK, "Amministratore riattivato.", "")
}

// validEmail — stesso sbarramento minimo di handler/helpers.go. Duplicato di
// proposito: quello è minuscolo e non esportato, ed esportarlo per tre righe
// legherebbe fra loro due pacchetti che non hanno altro in comune.
func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	return at > 0 && strings.Contains(s[at+1:], ".")
}
