package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	users       *repository.UserRepo
	jwtSecret   []byte
	isDev       bool
	mailer      Mailer
	frontendURL string
}

// NewAuthService — mailer può essere nil: in dev il login resta completabile
// col devToken restituito dall'handler, in produzione diventa un errore.
func NewAuthService(users *repository.UserRepo, secret string, isDev bool, mailer Mailer, frontendURL string) *AuthService {
	return &AuthService{
		users:       users,
		jwtSecret:   []byte(secret),
		isDev:       isDev,
		mailer:      mailer,
		frontendURL: frontendURL,
	}
}

// IsDev reports whether the service runs in development mode. Used to gate
// dev-only affordances — never let this be true in production.
func (s *AuthService) IsDev() bool { return s.isDev }

// NormalizeEmail porta l'indirizzo alla forma canonica con cui viene salvato.
// Chi fa rate limiting per email DEVE passare di qui, altrimenti basterebbe
// cambiare maiuscole a ogni richiesta per avere un secchiello nuovo.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SendMagicLink genera il token, lo spedisce per email e lo restituisce
// (l'handler lo espone solo in dev). Il link punta al frontend, che chiama
// /api/auth/verify e scambia il token per un JWT.
func (s *AuthService) SendMagicLink(ctx context.Context, email string) (string, error) {
	// Normalizzato qui una volta sola: il token viene salvato sotto questa
	// forma, quindi l'email deve partire verso lo stesso indirizzo.
	email = NormalizeEmail(email)
	token, err := s.users.StoreMagicLink(ctx, email)
	if err != nil {
		return "", err
	}
	link := s.magicLinkURL(token)

	if s.mailer == nil {
		if !s.isDev {
			return "", fmt.Errorf("no email provider configured: set RESEND_API_KEY")
		}
		fmt.Printf("[magic-link] %s → %s\n", email, link)
		return token, nil
	}

	if err := s.mailer.SendMagicLink(ctx, email, link); err != nil {
		return "", fmt.Errorf("send magic link: %w", err)
	}
	if s.isDev {
		fmt.Printf("[magic-link] sent to %s → %s\n", email, link)
	}
	return token, nil
}

func (s *AuthService) magicLinkURL(token string) string {
	return strings.TrimRight(s.frontendURL, "/") + "/login?token=" + url.QueryEscape(token)
}

func (s *AuthService) VerifyToken(ctx context.Context, token string) (*model.User, string, error) {
	email, err := s.users.VerifyMagicLink(ctx, token)
	if err != nil {
		return nil, "", fmt.Errorf("invalid or expired token")
	}
	user, err := s.users.FindOrCreate(ctx, email)
	if err != nil {
		return nil, "", err
	}
	jwt, err := s.IssueJWT(user)
	return user, jwt, err
}

func (s *AuthService) IssueJWT(u *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *AuthService) ParseJWT(tokenStr string) (userID, email string, err error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !t.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	claims := t.Claims.(jwt.MapClaims)
	userID, _ = claims["sub"].(string)
	email, _ = claims["email"].(string)
	return userID, email, nil
}
