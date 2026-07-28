package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const appleIssuer = "https://appleid.apple.com"

// appleSecretTTL — Apple rifiuta client secret con exp oltre sei mesi e ne
// impone la rotazione. Generandolo a ogni richiesta con vita brevissima il
// problema della rotazione semplicemente non esiste.
const appleSecretTTL = 5 * time.Minute

var appleEndpoint = oauth2.Endpoint{
	AuthURL:  appleIssuer + "/auth/authorize",
	TokenURL: appleIssuer + "/auth/token",
}

type AppleProvider struct {
	servicesID  string // client_id, il Services ID (non l'App ID)
	teamID      string
	keyID       string
	redirectURL string
	key         *ecdsa.PrivateKey
}

// NewAppleProvider — privateKey è il contenuto del file .p8 scaricato da Apple.
func NewAppleProvider(teamID, servicesID, keyID, privateKey, redirectURL string) (*AppleProvider, error) {
	key, err := parseApplePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return &AppleProvider{
		servicesID:  servicesID,
		teamID:      teamID,
		keyID:       keyID,
		redirectURL: redirectURL,
		key:         key,
	}, nil
}

func (p *AppleProvider) Name() string { return "apple" }

func (p *AppleProvider) AuthCodeURL(state, verifier string) string {
	return p.config("").AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		// Obbligatorio quando si chiedono gli scope name/email: il callback
		// arriverà come POST form-encoded, non come GET.
		oauth2.SetAuthURLParam("response_mode", "form_post"),
	)
}

func (p *AppleProvider) Exchange(ctx context.Context, code, verifier string) (Identity, error) {
	secret, err := p.clientSecret()
	if err != nil {
		return Identity{}, fmt.Errorf("apple client secret: %w", err)
	}

	tok, err := p.config(secret).Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("apple token exchange: %w", err)
	}

	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return Identity{}, fmt.Errorf("apple: no id_token in response")
	}
	claims, err := idTokenClaims(rawID)
	if err != nil {
		return Identity{}, fmt.Errorf("apple: %w", err)
	}
	if claimString(claims, "iss") != appleIssuer {
		return Identity{}, fmt.Errorf("apple: unexpected id_token issuer")
	}
	if err := checkAudience(claims, p.servicesID); err != nil {
		return Identity{}, fmt.Errorf("apple: %w", err)
	}

	email := claimString(claims, "email")
	if email == "" {
		return Identity{}, fmt.Errorf("apple: id_token has no email")
	}
	if !appleEmailVerified(claims) {
		return Identity{}, fmt.Errorf("apple: email not verified")
	}

	// Il nome non è mai nell'ID token: arriva una sola volta nel campo `user`
	// del form di callback, che l'handler passa a parte.
	return Identity{Email: email}, nil
}

func (p *AppleProvider) config(clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.servicesID,
		ClientSecret: clientSecret,
		RedirectURL:  p.redirectURL,
		Endpoint:     appleEndpoint,
		Scopes:       []string{"name", "email"},
	}
}

// clientSecret costruisce il JWT ES256 che Apple si aspetta al posto di un
// client secret statico.
func (p *AppleProvider) clientSecret() (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": p.teamID,
		"iat": now.Unix(),
		"exp": now.Add(appleSecretTTL).Unix(),
		"aud": appleIssuer,
		"sub": p.servicesID,
	})
	tok.Header["kid"] = p.keyID
	return tok.SignedString(p.key)
}

func parseApplePrivateKey(privateKey string) (*ecdsa.PrivateKey, error) {
	// Le variabili d'ambiente non conservano gli a capo veri: accettiamo sia il
	// PEM così com'è sia la forma con "\n" letterali.
	normalized := strings.ReplaceAll(strings.TrimSpace(privateKey), `\n`, "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("not a PEM block (expected the contents of the .p8 file)")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected an ECDSA key, got %T", parsed)
	}
	return key, nil
}

// Apple manda email_verified come booleano o come stringa "true", a seconda
// del caso; l'assenza del claim non è un fallimento.
func appleEmailVerified(claims map[string]any) bool {
	switch v := claims["email_verified"].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return true
	}
}

// ParseAppleUserField estrae il nome dal campo `user` del form di callback.
// Apple lo invia SOLO alla primissima autorizzazione: se non lo si prende
// adesso non lo si vedrà mai più.
func ParseAppleUserField(raw string) string {
	if raw == "" {
		return ""
	}
	var payload struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Name.FirstName + " " + payload.Name.LastName)
}
