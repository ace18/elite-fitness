package service

import (
	"context"
	"fmt"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	users     *repository.UserRepo
	jwtSecret []byte
	isDev     bool
}

func NewAuthService(users *repository.UserRepo, secret string, isDev bool) *AuthService {
	return &AuthService{users: users, jwtSecret: []byte(secret), isDev: isDev}
}

// IsDev reports whether the service runs in development mode. Used to gate
// dev-only affordances — never let this be true in production.
func (s *AuthService) IsDev() bool { return s.isDev }

// SendMagicLink genera il token e lo restituisce. Non esiste ancora un
// servizio email: in dev il token viene stampato (e restituito al chiamante,
// vedi handler), in produzione va spedito per email — finché non c'è, il
// login in produzione non è completabile.
func (s *AuthService) SendMagicLink(ctx context.Context, email string) (string, error) {
	token, err := s.users.StoreMagicLink(ctx, email)
	if err != nil {
		return "", err
	}
	if s.isDev {
		fmt.Printf("[magic-link] token for %s: %s\n", email, token)
	}
	// TODO: inviare l'email col link quando c'è un provider configurato.
	return token, nil
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
