package handler

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

var errMissingParams = errors.New("missing state or code")

type OAuthHandler struct {
	registry    *service.OAuthRegistry
	states      *repository.OAuthStateRepo
	users       *repository.UserRepo
	frontendURL string
}

func NewOAuthHandler(
	registry *service.OAuthRegistry,
	states *repository.OAuthStateRepo,
	users *repository.UserRepo,
	frontendURL string,
) *OAuthHandler {
	return &OAuthHandler{registry: registry, states: states, users: users, frontendURL: frontendURL}
}

// Providers elenca i provider configurati, così il frontend non disegna
// bottoni che non porterebbero da nessuna parte.
func (h *OAuthHandler) Providers(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{"providers": h.registry.Names()})
}

// Start avvia il flusso: è una navigazione a pagina intera, non una fetch,
// quindi la CORS non c'entra nulla.
func (h *OAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	provider, ok := h.registry.Get(name)
	if !ok {
		jsonError(w, "unknown provider", http.StatusNotFound)
		return
	}

	state, err := service.NewState()
	if err != nil {
		h.fail(w, r, "generate state", err)
		return
	}
	verifier, err := service.NewCodeVerifier()
	if err != nil {
		h.fail(w, r, "generate verifier", err)
		return
	}
	if err := h.states.Create(r.Context(), state, name, verifier); err != nil {
		h.fail(w, r, "store state", err)
		return
	}

	http.Redirect(w, r, provider.AuthCodeURL(state, verifier), http.StatusFound)
}

// Callback accetta sia GET (Google) sia POST form-encoded (Apple, che impone
// response_mode=form_post quando si chiedono gli scope name/email).
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	provider, ok := h.registry.Get(name)
	if !ok {
		jsonError(w, "unknown provider", http.StatusNotFound)
		return
	}

	// ParseForm unisce query string e body: un solo percorso per entrambi i verbi.
	if err := r.ParseForm(); err != nil {
		h.fail(w, r, "parse callback", err)
		return
	}

	// L'utente ha annullato dalla schermata del provider: non è un errore.
	if providerErr := r.Form.Get("error"); providerErr != "" {
		log.Printf("oauth %s: user aborted (%s)", name, providerErr)
		h.redirect(w, r, "/login?error=oauth_cancelled")
		return
	}

	state, code := r.Form.Get("state"), r.Form.Get("code")
	if state == "" || code == "" {
		h.fail(w, r, "callback", errMissingParams)
		return
	}

	verifier, err := h.states.Consume(r.Context(), state, name)
	if err != nil {
		// State sconosciuto, già consumato o scaduto — anche il replay finisce qui.
		h.fail(w, r, "consume state", err)
		return
	}

	identity, err := provider.Exchange(r.Context(), code, verifier)
	if err != nil {
		h.fail(w, r, "exchange", err)
		return
	}
	if identity.Name == "" {
		// Apple manda il nome solo alla prima autorizzazione, e nel form.
		identity.Name = service.ParseAppleUserField(r.Form.Get("user"))
	}

	// FindOrCreate/StoreMagicLink normalizzano l'email a monte.
	user, err := h.users.FindOrCreate(r.Context(), identity.Email)
	if err != nil {
		h.fail(w, r, "find or create user", err)
		return
	}
	if identity.Name != "" && user.Name == "" {
		if err := h.users.SetNameIfEmpty(r.Context(), user.ID, identity.Name); err != nil {
			// Il nome è un di più: non vale la pena far fallire il login.
			log.Printf("oauth %s: set name for %s: %v", name, user.ID, err)
		}
	}

	// Il JWT non viaggia nell'URL (finirebbe nella cronologia e nei log): si
	// riusa il token magic-link monouso, che il frontend sa già consumare.
	token, err := h.users.StoreMagicLink(r.Context(), identity.Email)
	if err != nil {
		h.fail(w, r, "mint handoff token", err)
		return
	}
	h.redirect(w, r, "/login?token="+url.QueryEscape(token))
}

func (h *OAuthHandler) fail(w http.ResponseWriter, r *http.Request, stage string, err error) {
	// Il motivo resta nei log: all'utente arriva solo "non ha funzionato".
	log.Printf("oauth %s: %s: %v", chi.URLParam(r, "provider"), stage, err)
	h.redirect(w, r, "/login?error=oauth_failed")
}

func (h *OAuthHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, strings.TrimRight(h.frontendURL, "/")+path, http.StatusFound)
}
