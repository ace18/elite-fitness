package service_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/db"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// L'allenamento proposto deve avanzare man mano che se ne registrano.
//
// Prima veniva scelto per giorno della settimana, con `workouts[0]` come
// ripiego quando nessun giorno combaciava — quindi di sabato (riposo nel 5×5)
// proponeva sempre il primo, e registrare una sessione non cambiava niente.
// Questo test fissa il comportamento: conta le sessioni della settimana di
// programma e propone la successiva in ordine di scheda.
//
//	TEST_DATABASE_URL="postgres://postgres@127.0.0.1:5433/elite?sslmode=disable" go test ./internal/service/
func TestNextWorkoutAdvancesWithLoggedSessions(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata — serve un Postgres di prova")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connessione: %v", err)
	}
	defer pool.Close()
	if _, err := db.Connect(ctx, url); err != nil {
		t.Fatalf("migrazioni: %v", err)
	}

	programs := repository.NewProgramRepo(pool)
	sessions := repository.NewSessionRepo(pool)
	svc := service.NewWorkoutService(programs, sessions)

	suffix := time.Now().Format("150405.000000")
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id`, "nextwo-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatal(err)
	}

	// Programma iniziato oggi con tre allenamenti: qualunque sia il giorno in
	// cui gira il test, la scelta non deve dipendere dal calendario.
	var programID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, name, goal, level, days_per_week, total_weeks, is_active, started_at)
		 VALUES ($1,'T','strength','beginner',3,4,TRUE,NOW()) RETURNING id`, userID).Scan(&programID); err != nil {
		t.Fatal(err)
	}
	// dayOfWeek deliberatamente "sbagliati" rispetto a oggi: se la selezione
	// tornasse a guardare il calendario, questo test se ne accorgerebbe.
	for i, name := range []string{"Primo", "Secondo", "Terzo"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO program_workouts (program_id, name, focus, day_of_week, order_in_week)
			 VALUES ($1,$2,'x',$3,$4)`, programID, name, i, i); err != nil {
			t.Fatal(err)
		}
	}

	logSession := func() {
		if _, err := pool.Exec(ctx,
			`INSERT INTO session_logs (user_id, program_id, name, completed_at) VALUES ($1,$2,'done',NOW())`,
			userID, programID); err != nil {
			t.Fatal(err)
		}
	}
	nextName := func() string {
		w, err := svc.BuildTodayWorkout(ctx, userID)
		if err != nil {
			t.Fatalf("BuildTodayWorkout: %v", err)
		}
		if w == nil {
			return "" // riposo
		}
		return w.Name
	}

	for i, want := range []string{"Primo", "Secondo", "Terzo"} {
		if got := nextName(); got != want {
			t.Fatalf("dopo %d sessioni: proposto %q, atteso %q", i, got, want)
		}
		logSession()
	}

	// Quota della settimana esaurita: niente altro da proporre.
	if got := nextName(); got != "" {
		t.Errorf("con tutte le sessioni fatte è stato proposto %q, atteso riposo", got)
	}
}
