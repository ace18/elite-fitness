package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Chiave arbitraria ma fissa per il lock advisory: serializza i boot
// concorrenti (deploy rolling) così due istanze non applicano la stessa
// migration insieme.
const migrationLockKey int64 = 8274611523

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return pool, nil
}

// runMigrations applica i file in migrations/ una sola volta ciascuno, in
// ordine di nome, registrandoli in schema_migrations. Ogni migration gira
// nella propria transazione insieme alla riga di registro: se fallisce a
// metà, non resta né applicata a metà né segnata come applicata.
//
// I file NON devono contenere BEGIN/COMMIT: la transazione la apre il runner,
// e un COMMIT interno chiuderebbe quella esterna prima che venga scritta la
// riga di registro. Per lo stesso motivo evitare le temp table ON COMMIT DROP
// se si vuole che il file resti eseguibile anche a mano via psql.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Il lock advisory è di sessione, quindi serve una connessione fissa per
	// tutta la durata: pool.Exec potrebbe prenderne una diversa ogni volta.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockKey); err != nil {
			log.Printf("migrations: advisory unlock: %v", err)
		}
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		  name       TEXT PRIMARY KEY,
		  checksum   TEXT NOT NULL,
		  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[string]string{}
	rows, err := conn.Query(ctx, `SELECT name, checksum FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var name, sum string
		if err := rows.Scan(&name, &sum); err != nil {
			rows.Close()
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[name] = sum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var ran int
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		data, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])

		if prev, ok := applied[name]; ok {
			// Warning e non errore: in fase di sviluppo capita di ritoccare una
			// migration già applicata, e bloccare il boot sarebbe peggio. Per un
			// ambiente stabile basta trasformare questo ramo in un return err.
			if prev != checksum {
				log.Printf("migrations: WARNING %s è cambiato dopo essere stato applicato "+
					"(registrato %s…, su disco %s…); NON viene rieseguito. "+
					"Le modifiche vanno in una nuova migration.",
					name, prev[:12], checksum[:12])
			}
			continue
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(data)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`,
			name, checksum); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
		log.Printf("migrations: applicata %s", name)
		ran++
	}

	if ran == 0 {
		log.Printf("migrations: nessuna nuova migration (%d già applicate)", len(applied))
	}
	return nil
}
