package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeIDToken costruisce un ID token con i claim dati. La firma è finta di
// proposito: nel flusso authorization code il token arriva dal token endpoint
// su TLS e non viene verificato (vedi idTokenClaims).
func fakeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".not-a-real-signature"
}

// googleWithStubEndpoint punta il provider a un finto token endpoint locale.
func googleWithStubEndpoint(t *testing.T, claims map[string]any) *GoogleProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token request: %v", err)
		}
		// Il code_verifier PKCE deve arrivare al token endpoint, altrimenti
		// Google rifiuterebbe lo scambio.
		if got := r.Form.Get("code_verifier"); got != "test-verifier" {
			t.Errorf("code_verifier = %q, want test-verifier", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			"token_type":   "Bearer",
			"id_token":     fakeIDToken(t, claims),
		})
	}))
	t.Cleanup(srv.Close)

	p := NewGoogleProvider("client-1", "secret", "https://elite.app/cb")
	p.cfg.Endpoint.TokenURL = srv.URL
	return p
}

func TestGoogleExchange(t *testing.T) {
	p := googleWithStubEndpoint(t, map[string]any{
		"iss":            "https://accounts.google.com",
		"aud":            "client-1",
		"email":          "alessio@example.com",
		"email_verified": true,
		"name":           "Alessio Crippa",
	})

	id, err := p.Exchange(context.Background(), "code", "test-verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Email != "alessio@example.com" {
		t.Errorf("Email = %q", id.Email)
	}
	if id.Name != "Alessio Crippa" {
		t.Errorf("Name = %q", id.Name)
	}
}

func TestGoogleExchangeRejects(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"iss": "https://accounts.google.com", "aud": "client-1",
			"email": "alessio@example.com", "email_verified": true,
		}
	}

	tests := map[string]struct {
		mutate  func(map[string]any)
		wantErr string
	}{
		// Un id_token emesso per un'altra app non deve valere come login qui.
		"foreign audience": {func(c map[string]any) { c["aud"] = "someone-else" }, "audience"},
		"wrong issuer":     {func(c map[string]any) { c["iss"] = "https://evil.example" }, "issuer"},
		// Senza questo controllo un account con email non verificata potrebbe
		// impersonare un utente registrato via magic link.
		"unverified email": {func(c map[string]any) { c["email_verified"] = false }, "not verified"},
		"no email":         {func(c map[string]any) { delete(c, "email") }, "no email"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			claims := base()
			tt.mutate(claims)
			p := googleWithStubEndpoint(t, claims)

			_, err := p.Exchange(context.Background(), "code", "test-verifier")
			if err == nil {
				t.Fatal("expected rejection, got a valid identity")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestGoogleAuthCodeURLCarriesPKCE(t *testing.T) {
	p := NewGoogleProvider("client-1", "secret", "https://elite.app/cb")
	url := p.AuthCodeURL("the-state", "the-verifier")

	for _, want := range []string{
		"state=the-state",
		"code_challenge=" + codeChallenge("the-verifier"),
		"code_challenge_method=S256",
		"scope=openid+email+profile",
	} {
		if !strings.Contains(url, want) {
			t.Errorf("auth URL missing %q\n  got: %s", want, url)
		}
	}
	// Il verifier non deve mai finire nell'URL mandato al browser.
	if strings.Contains(url, "the-verifier") {
		t.Error("the code_verifier leaked into the authorization URL")
	}
}
