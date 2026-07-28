package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// oauthStateTTL — quanto ha l'utente per completare il consenso dal provider.
const oauthStateTTL = 10 * time.Minute

type OAuthStateRepo struct {
	db *pgxpool.Pool
}

func NewOAuthStateRepo(db *pgxpool.Pool) *OAuthStateRepo { return &OAuthStateRepo{db: db} }

func (r *OAuthStateRepo) Create(ctx context.Context, state, provider, verifier string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO oauth_states (state, provider, code_verifier, expires_at) VALUES ($1,$2,$3,$4)`,
		state, provider, verifier, time.Now().Add(oauthStateTTL),
	)
	return err
}

// Consume restituisce il code_verifier e cancella la riga nella stessa query:
// uno state non è più riutilizzabile dopo il primo callback. Stesso schema
// one-shot di VerifyMagicLink.
func (r *OAuthStateRepo) Consume(ctx context.Context, state, provider string) (verifier string, err error) {
	err = r.db.QueryRow(ctx,
		`DELETE FROM oauth_states
		 WHERE state = $1 AND provider = $2 AND expires_at > NOW()
		 RETURNING code_verifier`,
		state, provider,
	).Scan(&verifier)
	return verifier, err
}

// DeleteExpired ripulisce gli state abbandonati (consenso mai completato).
func (r *OAuthStateRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at < NOW()`)
	return err
}
