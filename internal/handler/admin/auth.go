package admin

import (
	"net"
	"net/http"
	"time"

	"github.com/elitecoach/backend/internal/service"
)

// Limiti del form di accesso. Più stretti di quelli degli atleti
// (handler/auth.go): gli amministratori sono pochi e noti, quindi un limite
// basso non dà fastidio a nessuno di legittimo, mentre qui dietro c'è la vista
// su tutti gli iscritti.
const (
	loginPerIP    = 10
	loginIPWindow = time.Hour
)

type loginPageData struct {
	pageData
	Email string
	Sent  bool
	// DevToken compare solo in sviluppo, quando non c'è un provider di posta
	// configurato: senza, il pannello sarebbe irraggiungibile in locale.
	DevToken string
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	// Chi ha già una sessione valida non deve rivedere il form.
	if c, err := r.Cookie(sessionCookie); err == nil {
		if sess, err := h.svc.ParseJWT(c.Value); err == nil {
			if a, err := h.admins.FindByID(r.Context(), sess.AdminID); err == nil && a.Active() {
				http.Redirect(w, r, basePath+"/", http.StatusSeeOther)
				return
			}
		}
	}
	data := loginPageData{pageData: h.base(r, "Accesso", "")}
	h.rn.render(w, http.StatusOK, "login", data)
}

func (h *Handler) sendLoginLink(w http.ResponseWriter, r *http.Request) {
	// Niente controllo CSRF qui: la pagina di accesso è pubblica e chi la apre
	// non ha ancora una sessione da cui prendere un token. Non c'è nulla da
	// proteggere — l'unica azione possibile è farsi mandare un'email a un
	// indirizzo che deve già essere di un amministratore.
	if err := r.ParseForm(); err != nil {
		h.problem(w, r, http.StatusBadRequest, "Richiesta non valida.")
		return
	}
	email := service.NormalizeEmail(r.PostFormValue("email"))

	data := loginPageData{pageData: h.base(r, "Accesso", ""), Email: email, Sent: true}

	if email == "" {
		data.Sent = false
		data.Problem = "Inserisci il tuo indirizzo email."
		h.rn.render(w, http.StatusBadRequest, "login", data)
		return
	}

	if !h.loginLimiter.Allow(clientKey(r)) {
		data.Sent = false
		data.Problem = "Troppi tentativi. Riprova fra un'ora."
		h.rn.render(w, http.StatusTooManyRequests, "login", data)
		return
	}

	// SendLoginLink non distingue fra "inviata" e "non è un amministratore":
	// vedi il commento sul metodo. Qui si propaga solo un guasto vero.
	token, err := h.svc.SendLoginLink(r.Context(), email)
	if err != nil {
		h.fail(w, r, err, "invio link di accesso")
		return
	}
	if h.svc.IsDev() {
		data.DevToken = token
	}
	h.rn.render(w, http.StatusOK, "login", data)
}

func (h *Handler) verifyLogin(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.redirectToLogin(w, r)
		return
	}

	_, signed, err := h.svc.VerifyLoginToken(r.Context(), token)
	if err != nil {
		data := loginPageData{pageData: h.base(r, "Accesso", "")}
		data.Problem = "Link non valido o scaduto. Richiedine uno nuovo."
		h.rn.render(w, http.StatusUnauthorized, "login", data)
		return
	}

	h.setSession(w, signed)
	// 303 e non 302: il browser deve passare a GET, e soprattutto il token non
	// deve restare nella barra degli indirizzi — da lì finirebbe nella
	// cronologia e nel referer della prima risorsa esterna caricata.
	http.Redirect(w, r, basePath+"/", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.checkCSRF(w, r) {
		return
	}
	h.clearSession(w)
	http.Redirect(w, r, basePath+"/accesso", http.StatusSeeOther)
}

// clientKey — la chiave del rate limit. Legge RemoteAddr così com'è arrivato,
// come clientIP in handler/helpers.go: se il server gira dietro Cloudflare con
// TRUST_PROXY_HEADERS attivo, è il middleware CloudflareIP ad averlo già
// riscritto con l'IP vero. Leggere qui X-Forwarded-For senza saperlo renderebbe
// il limite aggirabile con un header falso.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
