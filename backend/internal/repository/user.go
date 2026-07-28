package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) FindOrCreate(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id, email, name, created_at`,
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) StoreMagicLink(ctx context.Context, email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := r.db.Exec(ctx,
		`INSERT INTO magic_link_tokens (email, token, expires_at) VALUES ($1, $2, $3)`,
		email, token, time.Now().Add(15*time.Minute),
	)
	return token, err
}

func (r *UserRepo) VerifyMagicLink(ctx context.Context, token string) (string, error) {
	var email string
	err := r.db.QueryRow(ctx,
		`UPDATE magic_link_tokens
		 SET used_at = NOW()
		 WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()
		 RETURNING email`,
		token,
	).Scan(&email)
	return email, err
}
