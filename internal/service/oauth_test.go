package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Vettore di test di RFC 7636 Appendix B: se il challenge PKCE è sbagliato il
// provider rifiuta lo scambio, ed è un errore difficile da diagnosticare.
func TestCodeChallengeMatchesRFC7636(t *testing.T) {
	const (
		verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		want     = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)
	if got := codeChallenge(verifier); got != want {
		t.Errorf("codeChallenge() = %q, want %q", got, want)
	}
}

// Il client secret di Apple non è verificabile in locale (Apple non accetta
// redirect http/localhost), quindi è l'unico pezzo che vale un test unitario.
func TestAppleClientSecret(t *testing.T) {
	key, pemKey := testP8Key(t)

	p, err := NewAppleProvider("TEAM123456", "com.elite.web", "KEY7890", pemKey, "https://elite.app/cb")
	if err != nil {
		t.Fatalf("NewAppleProvider: %v", err)
	}

	secret, err := p.clientSecret()
	if err != nil {
		t.Fatalf("clientSecret: %v", err)
	}

	tok, err := jwt.Parse(secret, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parse/verify client secret: %v", err)
	}
	if !tok.Valid {
		t.Fatal("client secret did not verify against the key that signed it")
	}
	if kid, _ := tok.Header["kid"].(string); kid != "KEY7890" {
		t.Errorf("kid header = %q, want KEY7890", kid)
	}
	if alg, _ := tok.Header["alg"].(string); alg != "ES256" {
		t.Errorf("alg header = %q, want ES256", alg)
	}

	claims := tok.Claims.(jwt.MapClaims)
	if iss, _ := claims["iss"].(string); iss != "TEAM123456" {
		t.Errorf("iss = %q, want the Team ID", iss)
	}
	if sub, _ := claims["sub"].(string); sub != "com.elite.web" {
		t.Errorf("sub = %q, want the Services ID", sub)
	}
	if aud, _ := claims["aud"].(string); aud != appleIssuer {
		t.Errorf("aud = %q, want %q", aud, appleIssuer)
	}

	// Apple rifiuta i secret con scadenza oltre sei mesi; noi stiamo su minuti.
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("exp: %v", err)
	}
	if d := time.Until(exp.Time); d <= 0 || d > appleSecretTTL+time.Minute {
		t.Errorf("exp is %v out, want (0, %v]", d, appleSecretTTL)
	}
}

func TestParseApplePrivateKeyRejectsGarbage(t *testing.T) {
	for name, input := range map[string]string{
		"empty":     "",
		"not PEM":   "just a string",
		"not a key": "-----BEGIN CERTIFICATE-----\nQUJD\n-----END CERTIFICATE-----",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseApplePrivateKey(input); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// Le variabili d'ambiente non conservano gli a capo: la forma con \n letterali
// deve funzionare come il PEM vero.
func TestParseApplePrivateKeyAcceptsEscapedNewlines(t *testing.T) {
	_, pemKey := testP8Key(t)
	escaped := ""
	for _, r := range pemKey {
		if r == '\n' {
			escaped += `\n`
			continue
		}
		escaped += string(r)
	}
	if _, err := parseApplePrivateKey(escaped); err != nil {
		t.Errorf("escaped-newline key rejected: %v", err)
	}
}

func TestParseAppleUserField(t *testing.T) {
	tests := map[string]struct{ in, want string }{
		"full name":   {`{"name":{"firstName":"Alessio","lastName":"Crippa"},"email":"a@b.c"}`, "Alessio Crippa"},
		"first only":  {`{"name":{"firstName":"Alessio","lastName":""}}`, "Alessio"},
		"absent":      {"", ""},
		"no name key": {`{"email":"a@b.c"}`, ""},
		"malformed":   {"{not json", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ParseAppleUserField(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegistryNamesOnlyIncludesConfigured(t *testing.T) {
	r := NewOAuthRegistry(NewGoogleProvider("id", "secret", "https://elite.app/cb"))
	if got := r.Names(); len(got) != 1 || got[0] != "google" {
		t.Errorf("Names() = %v, want [google]", got)
	}
	if _, ok := r.Get("apple"); ok {
		t.Error("apple resolved despite not being configured")
	}
	if !r.Enabled() {
		t.Error("Enabled() = false with one provider registered")
	}

	empty := NewOAuthRegistry()
	if empty.Enabled() || len(empty.Names()) != 0 {
		t.Error("an empty registry should report nothing enabled")
	}
}

func TestCheckAudience(t *testing.T) {
	if err := checkAudience(map[string]any{"aud": "client-1"}, "client-1"); err != nil {
		t.Errorf("string aud: %v", err)
	}
	if err := checkAudience(map[string]any{"aud": []any{"other", "client-1"}}, "client-1"); err != nil {
		t.Errorf("array aud: %v", err)
	}
	if err := checkAudience(map[string]any{"aud": "attacker"}, "client-1"); err == nil {
		t.Error("mismatched aud accepted — an id_token for another client would pass")
	}
	if err := checkAudience(map[string]any{}, "client-1"); err == nil {
		t.Error("missing aud accepted")
	}
}

// testP8Key restituisce una chiave P-256 e il suo PEM PKCS#8, cioè la stessa
// forma del file .p8 di Apple.
func testP8Key(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}
