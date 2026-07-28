package service

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// Endpoint dichiarato a mano invece di usare oauth2/google: quel package tira
// dentro cloud.google.com/go/compute/metadata per il rilevamento delle
// credenziali GCE, che qui non serve a niente.
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// Google accetta due issuer per gli ID token, entrambi legittimi.
var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

type GoogleProvider struct {
	cfg *oauth2.Config
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     googleEndpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}}
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) AuthCodeURL(state, verifier string) string {
	return p.cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *GoogleProvider) Exchange(ctx context.Context, code, verifier string) (Identity, error) {
	tok, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("google token exchange: %w", err)
	}

	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return Identity{}, fmt.Errorf("google: no id_token in response")
	}
	claims, err := idTokenClaims(rawID)
	if err != nil {
		return Identity{}, fmt.Errorf("google: %w", err)
	}
	if !googleIssuers[claimString(claims, "iss")] {
		return Identity{}, fmt.Errorf("google: unexpected id_token issuer")
	}
	if err := checkAudience(claims, p.cfg.ClientID); err != nil {
		return Identity{}, fmt.Errorf("google: %w", err)
	}

	email := claimString(claims, "email")
	if email == "" {
		return Identity{}, fmt.Errorf("google: id_token has no email")
	}
	// Un account Google può avere un'email non verificata (rari casi di account
	// aziendali): accettarla permetterebbe di impersonare un utente magic-link.
	if verified, ok := claims["email_verified"].(bool); ok && !verified {
		return Identity{}, fmt.Errorf("google: email not verified")
	}

	return Identity{Email: email, Name: claimString(claims, "name")}, nil
}
