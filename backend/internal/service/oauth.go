package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Identity è quel poco che ci serve da un provider: l'email verificata (è la
// nostra chiave d'identità, come per il magic link) e, se c'è, il nome.
type Identity struct {
	Email string
	Name  string
}

// OAuthProvider — aggiungere Microsoft/GitHub domani significa un file nuovo
// che implementa questi tre metodi, più una riga in NewOAuthRegistry.
type OAuthProvider interface {
	Name() string
	AuthCodeURL(state, verifier string) string
	// Exchange scambia il code per i token e ne estrae l'identità.
	Exchange(ctx context.Context, code, verifier string) (Identity, error)
}

// OAuthRegistry contiene solo i provider effettivamente configurati: se le
// variabili d'ambiente di Apple mancano, il bottone Apple non esiste proprio.
type OAuthRegistry struct {
	providers map[string]OAuthProvider
}

func NewOAuthRegistry(providers ...OAuthProvider) *OAuthRegistry {
	m := make(map[string]OAuthProvider, len(providers))
	for _, p := range providers {
		if p != nil {
			m[p.Name()] = p
		}
	}
	return &OAuthRegistry{providers: m}
}

func (r *OAuthRegistry) Get(name string) (OAuthProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Names restituisce i provider abilitati in ordine stabile, per il frontend.
func (r *OAuthRegistry) Names() []string {
	// Ordine di presentazione voluto sulla schermata di login, non alfabetico.
	order := []string{"apple", "google"}
	out := make([]string, 0, len(r.providers))
	for _, name := range order {
		if _, ok := r.providers[name]; ok {
			out = append(out, name)
		}
	}
	// Eventuali provider aggiunti in futuro e non ancora in `order`.
	for name := range r.providers {
		if name != "apple" && name != "google" {
			out = append(out, name)
		}
	}
	return out
}

func (r *OAuthRegistry) Enabled() bool { return len(r.providers) > 0 }

// randomURLSafe genera n byte casuali in base64url — usato sia per lo state
// (CSRF) sia per il code_verifier PKCE.
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func NewState() (string, error)        { return randomURLSafe(32) }
func NewCodeVerifier() (string, error) { return randomURLSafe(32) }

// codeChallenge — S256 come richiesto da PKCE (RFC 7636).
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// idTokenClaims decodifica il payload di un ID token SENZA verificarne la
// firma. È corretto in questo flusso: il token arriva direttamente dal token
// endpoint del provider su TLS, in cambio del nostro client secret, quindi non
// è passato per il browser (OIDC Core §3.1.3.7). Chi lo chiama deve comunque
// controllare `iss` e `aud`.
func idTokenClaims(idToken string) (map[string]any, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("id_token payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("id_token claims: %w", err)
	}
	return claims, nil
}

// checkAudience verifica che l'ID token sia stato emesso per noi. `aud` può
// essere una stringa o un array, da spec.
func checkAudience(claims map[string]any, clientID string) error {
	switch aud := claims["aud"].(type) {
	case string:
		if aud == clientID {
			return nil
		}
	case []any:
		for _, a := range aud {
			if s, ok := a.(string); ok && s == clientID {
				return nil
			}
		}
	}
	return fmt.Errorf("id_token audience mismatch")
}

func claimString(claims map[string]any, key string) string {
	s, _ := claims[key].(string)
	return s
}
