// Package admin è il pannello di gestione, reso dal server.
//
// Niente JSON e niente bundle: le pagine sono html/template compilati dentro il
// binario. La scelta è che questo è un pannello di lettura per una manciata di
// persone sedute a una scrivania, non un'app offline su un telefono. Non c'è
// niente da installare, niente da sincronizzare, e nessuna API pubblica da
// mantenere: i template chiamano direttamente i repository, che prendono già
// l'id dell'utente come parametro e quindi funzionano identici sia che a
// chiamarli sia l'atleta sia il pannello.
//
// La separazione dall'API degli atleti è netta e voluta:
//   - sessione in un cookie HttpOnly con Path=/admin, quindi non viene mai
//     spedito a /api/* e l'API resta solo-bearer, cioè immune al CSRF;
//   - JWT con `aud` suo (vedi service/admin.go), quindi un token da atleta non
//     vale qui nemmeno essendo firmato con lo stesso segreto;
//   - amministratori in una tabella loro, dove non si entra registrandosi.
package admin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/ratelimit"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// basePath — dove il pannello è montato. I link nei template lo antepongono
// (funzione `url` nei dati di pagina) perché chi.Mount toglie il prefisso in
// ingresso ma non lo rimette negli URL che scriviamo noi.
const basePath = "/admin"

// sessionCookie — il nome del cookie di sessione.
const sessionCookie = "elite_admin"

// csrfField — il campo nascosto che ogni form deve rimandare.
const csrfField = "_csrf"

type ctxKey string

const adminCtxKey ctxKey = "admin"
const sessionCtxKey ctxKey = "adminSession"

type Handler struct {
	svc      *service.AdminService
	admins   *repository.AdminRepo
	programs *repository.ProgramRepo
	sessions *repository.SessionRepo
	catalog  *repository.CatalogRepo
	rn       *renderer
	// loginLimiter frena il form di accesso: è l'unica pagina pubblica del
	// pannello e ogni invio manda un'email vera.
	loginLimiter *ratelimit.Limiter
	// secureCookies — Secure sul cookie di sessione. Spento in sviluppo, dove
	// il pannello gira su http://localhost e un cookie Secure non verrebbe mai
	// mandato indietro, rendendo impossibile l'accesso.
	secureCookies bool
}

func New(
	svc *service.AdminService,
	admins *repository.AdminRepo,
	programs *repository.ProgramRepo,
	sessions *repository.SessionRepo,
	catalog *repository.CatalogRepo,
	isDev bool,
) (*Handler, error) {
	rn, err := newRenderer(isDev)
	if err != nil {
		return nil, err
	}
	return &Handler{
		svc:           svc,
		admins:        admins,
		programs:      programs,
		sessions:      sessions,
		catalog:       catalog,
		rn:            rn,
		loginLimiter:  ratelimit.New(loginPerIP, loginIPWindow),
		secureCookies: !isDev,
	}, nil
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/statico/admin.css", h.css)

	r.Get("/accesso", h.loginPage)
	r.Post("/accesso", h.sendLoginLink)
	r.Get("/accesso/verifica", h.verifyLogin)

	r.Group(func(r chi.Router) {
		r.Use(h.requireAdmin)

		r.Get("/", h.roster)
		r.Post("/esci", h.logout)
		r.Get("/atleti/{id}", h.athlete)
		r.Post("/atleti/{id}/programma", h.assignProgram)
		r.Get("/atleti/{id}/sessioni/{sessionID}", h.session)
		r.Get("/amministratori", h.adminsPage)
		r.Post("/amministratori", h.createAdmin)
		r.Post("/amministratori/{id}/stato", h.toggleAdmin)

		// Libreria esercizi.
		r.Get("/esercizi", h.exercises)
		r.Post("/esercizi", h.createExercise)
		r.Get("/esercizi/{id}", h.editExercise)
		r.Post("/esercizi/{id}", h.updateExercise)
		r.Post("/esercizi/{id}/elimina", h.deleteExercise)

		// Catalogo programmi: la template, i suoi giorni, e gli esercizi dentro
		// ogni giorno. Tre livelli, tre pagine.
		r.Get("/programmi", h.templates)
		r.Post("/programmi", h.createTemplate)
		r.Get("/programmi/{id}", h.templateDetail)
		r.Post("/programmi/{id}", h.updateTemplate)
		r.Post("/programmi/{id}/stato", h.archiveTemplate)
		r.Post("/programmi/{id}/elimina", h.deleteTemplate)
		r.Post("/programmi/{id}/giorni", h.createDay)
		r.Get("/programmi/{id}/giorni/{dayID}", h.dayDetail)
		r.Post("/programmi/{id}/giorni/{dayID}", h.updateDay)
		r.Post("/programmi/{id}/giorni/{dayID}/elimina", h.deleteDay)
		r.Post("/programmi/{id}/giorni/{dayID}/esercizi", h.addExerciseRow)
		r.Post("/programmi/{id}/giorni/{dayID}/esercizi/{rowID}", h.updateExerciseRow)
		r.Post("/programmi/{id}/giorni/{dayID}/esercizi/{rowID}/elimina", h.deleteExerciseRow)
		r.Post("/programmi/{id}/giorni/{dayID}/esercizi/{rowID}/sposta", h.moveExerciseRow)
	})

	return r
}

// ---- dati comuni a ogni pagina ---------------------------------------------

// pageData sta alla base dei dati di ogni pagina. I template ci arrivano con
// `.Base`, così ogni struttura di pagina la incorpora e non deve ricordarsi di
// riempire il menù o il token CSRF.
type pageData struct {
	Title   string
	Nav     string // quale voce del menù è attiva
	URL     string // basePath, per comporre i link
	Admin   *model.Admin
	CSRF    string
	Flash   string // messaggio verde
	Problem string // messaggio rosso
}

func (h *Handler) base(r *http.Request, title, nav string) pageData {
	d := pageData{Title: title, Nav: nav, URL: basePath}
	if a, ok := r.Context().Value(adminCtxKey).(*model.Admin); ok {
		d.Admin = a
	}
	if s, ok := r.Context().Value(sessionCtxKey).(service.AdminSession); ok {
		d.CSRF = s.CSRF
	}
	return d
}

// ---- middleware ------------------------------------------------------------

// requireAdmin convalida il cookie e ricarica l'amministratore dal database a
// ogni richiesta.
//
// La rilettura non è uno spreco da togliere: è quello che rende immediata la
// disattivazione. Fidandosi del solo JWT, un accesso revocato resterebbe buono
// fino alla scadenza della sessione — e revocare qualcuno è esattamente il
// momento in cui non si vuole aspettare dodici ore.
func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			h.redirectToLogin(w, r)
			return
		}
		sess, err := h.svc.ParseJWT(c.Value)
		if err != nil {
			h.clearSession(w)
			h.redirectToLogin(w, r)
			return
		}
		admin, err := h.admins.FindByID(r.Context(), sess.AdminID)
		if err != nil || !admin.Active() {
			h.clearSession(w)
			h.redirectToLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), adminCtxKey, admin)
		ctx = context.WithValue(ctx, sessionCtxKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// checkCSRF va chiamata all'inizio di ogni POST. Il cookie è SameSite=Lax, che
// da solo già ferma le POST cross-site nei browser aggiornati; questo è il
// secondo strato, per non dipendere solo da quello.
func (h *Handler) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		h.problem(w, r, http.StatusBadRequest, "Richiesta non valida.")
		return false
	}
	sess, ok := r.Context().Value(sessionCtxKey).(service.AdminSession)
	if !ok || !sess.ValidCSRF(r.PostFormValue(csrfField)) {
		h.problem(w, r, http.StatusForbidden,
			"Sessione scaduta o richiesta non valida. Ricarica la pagina e riprova.")
		return false
	}
	return true
}

// ---- sessione --------------------------------------------------------------

func (h *Handler) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:  sessionCookie,
		Value: token,
		// Path=/admin: il cookie non viene mai spedito a /api/*, quindi l'API
		// degli atleti resta autenticata solo dal bearer token e non diventa
		// attaccabile via CSRF per il fatto che esiste questo pannello.
		Path:     basePath,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(12 * time.Hour / time.Second),
	})
}

func (h *Handler) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     basePath,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (h *Handler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, basePath+"/accesso", http.StatusSeeOther)
}

// ---- esiti dopo una POST ---------------------------------------------------

// esiti — i messaggi che sopravvivono a un redirect.
//
// Ogni POST che cambia qualcosa finisce con Post/Redirect/Get invece di
// ridisegnare la pagina sul posto: ricaricando dopo una POST il browser rimanda
// gli stessi dati, e per un'assegnazione di programma significa assegnarlo una
// seconda volta, con la settimana dell'atleta che riparte da uno. Il messaggio
// non può viaggiare nel corpo di un redirect, quindi viaggia nell'URL come
// chiave: il testo resta sul server e non si può falsificare dalla barra degli
// indirizzi.
var esiti = map[string]string{
	"programma-assegnato":   "Programma assegnato. L'atleta lo vedrà al prossimo avvio dell'app.",
	"esercizio-creato":      "Esercizio aggiunto alla libreria.",
	"esercizio-modificato":  "Esercizio aggiornato.",
	"esercizio-eliminato":   "Esercizio eliminato dalla libreria.",
	"template-creata":       "Programma creato. Ora definiscine i giorni.",
	"template-modificata":   "Programma aggiornato. I programmi già assegnati non cambiano: sono copie.",
	"template-archiviata":   "Programma archiviato: non comparirà più fra quelli assegnabili.",
	"template-ripristinata": "Programma ripristinato.",
	"template-eliminata":    "Programma eliminato.",
	"giorno-creato":         "Giorno aggiunto.",
	"giorno-modificato":     "Giorno aggiornato.",
	"giorno-eliminato":      "Giorno eliminato.",
	"riga-aggiunta":         "Esercizio aggiunto al giorno.",
	"riga-modificata":       "Riga aggiornata.",
	"riga-eliminata":        "Esercizio tolto dal giorno.",
}

func esitoMessage(key string) string { return esiti[key] }

// redirectWith rimanda a una pagina con un esito da mostrare.
func (h *Handler) redirectWith(w http.ResponseWriter, r *http.Request, path, esito string) {
	if esito != "" {
		path += "?esito=" + esito
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}

// plural sceglie fra singolare e plurale. `many` deve contenere un %d.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return fmt.Sprintf(many, n)
}

// ---- errori ----------------------------------------------------------------

// problem mostra una pagina d'errore. I messaggi sono scritti per chi legge,
// non copiati dall'errore vero: quello finisce nei log (vedi fail).
func (h *Handler) problem(w http.ResponseWriter, r *http.Request, status int, msg string) {
	data := struct {
		pageData
		Message string
	}{h.base(r, "Errore", ""), msg}
	h.rn.render(w, status, "error", data)
}

// fail registra l'errore vero e ne mostra uno generico.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	log.Printf("admin: %s: %v", what, err)
	h.problem(w, r, http.StatusInternalServerError,
		"Qualcosa non ha funzionato. Riprova fra un momento.")
}

// logf — per quello che va saputo ma non impedisce di servire la pagina.
func (h *Handler) logf(format string, args ...any) {
	log.Printf("admin: "+format, args...)
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request, msg string) {
	h.problem(w, r, http.StatusNotFound, msg)
}

// ---- statico ---------------------------------------------------------------

func (h *Handler) css(w http.ResponseWriter, r *http.Request) {
	// Come per i template: in sviluppo si legge da disco, se no si servirebbe
	// per sempre la copia catturata dall'embed al momento della compilazione.
	b, err := h.rn.readAsset("static/admin.css")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	// Il foglio di stile è compilato nel binario, quindi cambia solo con un
	// deploy — ma non ha l'hash nel nome, quindi non può essere immutabile. In
	// sviluppo non va cachato affatto, se no le modifiche non si vedono.
	if h.rn.devFS != nil {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	w.Write(b)
}
